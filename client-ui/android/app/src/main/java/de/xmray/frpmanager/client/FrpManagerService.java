package de.xmray.frpmanager.client;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.ConnectivityManager;
import android.net.LinkProperties;
import android.net.Network;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.PowerManager;

import androidx.annotation.Nullable;
import androidx.core.content.ContextCompat;
import androidx.core.app.NotificationCompat;

import com.getcapacitor.JSObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.net.InetAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayDeque;
import java.util.Arrays;
import java.util.Deque;
import java.util.List;
import java.util.Map;
import java.util.TimeZone;
import java.util.UUID;
import java.util.regex.Pattern;

public class FrpManagerService extends Service {
    private static final String CHANNEL_ID = "frp_manager_tunnel";
    private static final int NOTIFICATION_ID = 1001;
    private static final String ACTION_START = "de.xmray.frpmanager.client.START";
    private static final String ACTION_RESTART = "de.xmray.frpmanager.client.RESTART";
    private static final String PREFS = "frp_manager_service";
    private static final String CONFIG_FILE = ".env";
    private static final Object PROCESS_LOCK = new Object();
    private static final Deque<String> LOGS = new ArrayDeque<>();
    private static final long[] RESTART_DELAYS_MS = {1_000L, 3_000L, 10_000L, 30_000L};
    private static final Pattern ANSI_ESCAPE = Pattern.compile("\\u001B\\[[0-?]*[ -/]*[@-~]");
    private static Process process;
    private static String runtimeState = "stopped";
    private final Handler restartHandler = new Handler(Looper.getMainLooper());
    private ConnectivityManager connectivityManager;
    private ConnectivityManager.NetworkCallback networkCallback;
    private Runnable restartRunnable;
    private boolean destroyed;
    private int restartAttempt;
    private String processDnsServers = "";

    public static void saveProfile(Context context, JSObject profile) throws Exception {
        SharedPreferences preferences = context.getSharedPreferences(PREFS, MODE_PRIVATE);
        String globalSecret = preferences.getString("globalSecret", "");
        if (globalSecret.trim().isEmpty()) {
            globalSecret = UUID.randomUUID() + UUID.randomUUID().toString();
        }
        preferences.edit()
                .putString("globalSecret", globalSecret)
                .putBoolean("configured", true)
                .apply();

        List<String> lines = Arrays.asList(
                "APP_GLOBAL_SECRET=" + dotenv(globalSecret),
                "APP_AUTO_UPDATE=false",
                "CLIENT_FEATURES_ENABLE_FUNCTIONS=false",
                "CLIENT_WORKER_WORKERD_WORK_DIR=" + dotenv(
                        new File(context.getFilesDir(), "workerd").getAbsolutePath()
                ),
                "CLIENT_ID=" + dotenv(profile.optString("clientId")),
                "CLIENT_SECRET=" + dotenv(profile.optString("secret")),
                "CLIENT_JOIN_TOKEN=" + dotenv(profile.optString("joinToken")),
                "CLIENT_ENROLLMENT_ATTEMPT=" + dotenv(java.util.UUID.randomUUID().toString()),
                "CLIENT_API_URL=" + dotenv(profile.optString("apiUrl")),
                "CLIENT_RPC_URL=" + dotenv(profile.optString("rpcUrl")),
                ""
        );
        File config = new File(context.getFilesDir(), CONFIG_FILE);
        try (FileOutputStream output = new FileOutputStream(config, false)) {
            output.write(joinStrings(lines, "\n").getBytes(StandardCharsets.UTF_8));
        }
        config.setReadable(false, false);
        config.setWritable(false, false);
        config.setReadable(true, true);
        config.setWritable(true, true);
    }

    public static void clearProfile(Context context) {
        context.getSharedPreferences(PREFS, MODE_PRIVATE).edit().clear().apply();
        File config = new File(context.getFilesDir(), CONFIG_FILE);
        if (config.exists() && !config.delete()) {
            appendLog("Unable to remove the saved client configuration.");
        }
    }

    public static void setAutoStart(Context context, boolean enabled) {
        context.getSharedPreferences(PREFS, MODE_PRIVATE).edit().putBoolean("autoStart", enabled).apply();
    }

    public static boolean shouldAutoStart(Context context) {
        SharedPreferences preferences = context.getSharedPreferences(PREFS, MODE_PRIVATE);
        return preferences.getBoolean("configured", false) && preferences.getBoolean("autoStart", false);
    }

    public static void start(Context context, boolean restart) {
        synchronized (PROCESS_LOCK) {
            runtimeState = restart ? "restarting" : "starting";
        }
        Intent intent = new Intent(context, FrpManagerService.class);
        intent.setAction(restart ? ACTION_RESTART : ACTION_START);
        ContextCompat.startForegroundService(context, intent);
    }

    public static void stop(Context context) {
        context.stopService(new Intent(context, FrpManagerService.class));
        synchronized (PROCESS_LOCK) {
            stopProcessLocked();
            runtimeState = isConfigured(context) ? "stopped" : "not installed";
        }
    }

    public static JSObject runtimeStatus(Context context) {
        JSObject status = new JSObject();
        status.put("platform", "android");
        status.put("serviceStatus", currentState(context));
        status.put("binaryPath", binaryPath(context));
        status.put("dataDir", context.getFilesDir().getAbsolutePath());
        status.put("batteryOptimizationExempt", isBatteryOptimizationExempt(context));
        return status;
    }

    public static String recentLogs() {
        synchronized (LOGS) {
            return joinStrings(LOGS, "\n");
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        destroyed = false;
        createNotificationChannel();
        registerNetworkCallback();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        startForeground(NOTIFICATION_ID, notification("Starting client connection"));
        String action = intent == null ? ACTION_START : intent.getAction();
        boolean restart = ACTION_RESTART.equals(action);
        launchStart(restart);
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        destroyed = true;
        restartHandler.removeCallbacksAndMessages(null);
        unregisterNetworkCallback();
        synchronized (PROCESS_LOCK) {
            stopProcessLocked();
            if (!"error".equals(runtimeState)) {
                runtimeState = isConfigured(this) ? "stopped" : "not installed";
            }
        }
        super.onDestroy();
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    private void launchStart(boolean restart) {
        Thread worker = new Thread(() -> startProcess(restart), "frp-manager-android-start");
        worker.setDaemon(true);
        worker.start();
    }

    private void startProcess(boolean restart) {
        boolean waitForNetwork = false;
        synchronized (PROCESS_LOCK) {
            cancelScheduledRestartLocked();
            if (restart) {
                stopProcessLocked();
            } else if (isAlive(process)) {
                return;
            }

            File binary = new File(binaryPath(this));
            File config = new File(getFilesDir(), CONFIG_FILE);
            if (!binary.isFile()) {
                failAndStop("Bundled frp-manager core is missing for this CPU architecture.");
                return;
            }
            if (!config.isFile()) {
                failAndStop("Client configuration is not installed.");
                return;
            }

            String dnsServers = activeDnsServers();
            if (dnsServers.isEmpty()) {
                runtimeState = "waiting for network";
                updateNotification("Waiting for an active network");
                waitForNetwork = true;
            }
            if (waitForNetwork) {
                processDnsServers = "";
            } else {
                try {
                    ProcessBuilder builder = new ProcessBuilder(binary.getAbsolutePath(), "client");
                    builder.directory(getFilesDir());
                    builder.redirectErrorStream(true);
                    Map<String, String> environment = builder.environment();
                    environment.put("APP_AUTO_UPDATE", "false");
                    environment.put("CLIENT_FEATURES_ENABLE_FUNCTIONS", "false");
                    environment.put(
                            "CLIENT_WORKER_WORKERD_WORK_DIR",
                            new File(getFilesDir(), "workerd").getAbsolutePath()
                    );
                    environment.put("FRP_MANAGER_DNS_SERVERS", dnsServers);
                    environment.put("HOME", getFilesDir().getAbsolutePath());
                    environment.put("XDG_CONFIG_HOME", getFilesDir().getAbsolutePath());
                    environment.put("TMPDIR", getCacheDir().getAbsolutePath());
                    environment.put("TZ", TimeZone.getDefault().getID());
                    process = builder.start();
                    processDnsServers = dnsServers;
                    runtimeState = "connecting";
                    appendLog("frp-manager core started using Android system DNS.");
                    updateNotification("Connecting to the manager");
                    drainOutput(process);
                } catch (Exception error) {
                    failAndStop("Unable to start frp-manager core: " + error.getMessage());
                }
            }
        }
        if (waitForNetwork) {
            scheduleRestart("No active Android DNS server is available");
        }
    }

    private void drainOutput(Process startedProcess) {
        Thread reader = new Thread(() -> {
            try {
                try (BufferedReader input = new BufferedReader(new InputStreamReader(
                        startedProcess.getInputStream(), StandardCharsets.UTF_8))) {
                    String line;
                    while ((line = input.readLine()) != null) {
                        handleCoreLog(startedProcess, line);
                    }
                }
            } catch (Exception error) {
                appendLog("Log stream ended: " + error.getMessage());
            }
        }, "frp-manager-android-log");
        reader.setDaemon(true);
        reader.start();

        Thread waiter = new Thread(() -> {
            try {
                int exitCode = startedProcess.waitFor();
                appendLog("frp-manager core exited with code " + exitCode + ".");
                boolean restart = false;
                boolean stopService = false;
                synchronized (PROCESS_LOCK) {
                    if (process == startedProcess) {
                        process = null;
                        processDnsServers = "";
                        if (!destroyed && shouldAutoStart(this)) {
                            runtimeState = "restarting";
                            restart = true;
                        } else {
                            runtimeState = exitCode == 0 ? "stopped" : "error";
                            stopService = true;
                        }
                    }
                }
                if (restart) {
                    scheduleRestart("The core process exited unexpectedly");
                } else if (stopService) {
                    updateNotification(exitCode == 0 ? "Client connection stopped" : "Client connection failed");
                    stopSelf();
                }
            } catch (InterruptedException error) {
                Thread.currentThread().interrupt();
            }
        }, "frp-manager-android-wait");
        waiter.setDaemon(true);
        waiter.start();
    }

    private void handleCoreLog(Process startedProcess, String line) {
        String cleanLine = ANSI_ESCAPE.matcher(line).replaceAll("");
        appendLog(cleanLine);
        String normalized = cleanLine.toLowerCase();
        String notificationText = null;
        synchronized (PROCESS_LOCK) {
            if (process != startedProcess) {
                return;
            }
            if (normalized.contains("client registration succeeded")
                    || normalized.contains("client get server register envent success")
                    || normalized.contains("client get server register event success")) {
                runtimeState = "connected";
                restartAttempt = 0;
                notificationText = "Connected to the manager";
            } else if (normalized.contains("new tls client certificate failed")
                    || normalized.contains("client registration failed")
                    || normalized.contains("cannot receive, sleep 3s and return")
                    || normalized.contains("cannot recv, sleep 3s and retry")) {
                runtimeState = "connecting";
                notificationText = "Reconnecting to the manager";
            }
        }
        if (notificationText != null) {
            updateNotification(notificationText);
        }
    }

    private void scheduleRestart(String reason) {
        final long delay;
        final Runnable task;
        synchronized (PROCESS_LOCK) {
            if (destroyed || !shouldAutoStart(this) || restartRunnable != null) {
                return;
            }
            int delayIndex = Math.min(restartAttempt, RESTART_DELAYS_MS.length - 1);
            delay = RESTART_DELAYS_MS[delayIndex];
            restartAttempt++;
            if (!"waiting for network".equals(runtimeState)) {
                runtimeState = "restarting";
            }
            task = () -> {
                synchronized (PROCESS_LOCK) {
                    if (restartRunnable != null) {
                        restartRunnable = null;
                    }
                    if (destroyed || !shouldAutoStart(this)) {
                        return;
                    }
                }
                launchStart(false);
            };
            restartRunnable = task;
        }
        appendLog(reason + "; retrying in " + (delay / 1000) + " seconds.");
        if (!"waiting for network".equals(runtimeState)) {
            updateNotification("Restarting client connection");
        }
        restartHandler.postDelayed(task, delay);
    }

    private void cancelScheduledRestartLocked() {
        if (restartRunnable != null) {
            restartHandler.removeCallbacks(restartRunnable);
            restartRunnable = null;
        }
    }

    private String activeDnsServers() {
        if (connectivityManager == null) {
            return "";
        }
        Network network = connectivityManager.getActiveNetwork();
        if (network == null) {
            return "";
        }
        return dnsServers(connectivityManager.getLinkProperties(network));
    }

    private static String dnsServers(LinkProperties linkProperties) {
        if (linkProperties == null) {
            return "";
        }
        List<String> servers = new java.util.ArrayList<>();
        for (InetAddress address : linkProperties.getDnsServers()) {
            String host = address.getHostAddress();
            if (host != null && !host.trim().isEmpty()) {
                servers.add(host);
            }
        }
        return joinStrings(servers, ",");
    }

    private void registerNetworkCallback() {
        connectivityManager = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
        if (connectivityManager == null) {
            return;
        }
        networkCallback = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onLinkPropertiesChanged(Network network, LinkProperties linkProperties) {
                String dns = dnsServers(linkProperties);
                boolean restart = false;
                boolean resume = false;
                synchronized (PROCESS_LOCK) {
                    if (!dns.isEmpty() && isAlive(process) && !dns.equals(processDnsServers)) {
                        restart = true;
                    } else if (!dns.isEmpty() && !isAlive(process)
                            && "waiting for network".equals(runtimeState)
                            && shouldAutoStart(FrpManagerService.this)) {
                        resume = true;
                    }
                }
                if (restart) {
                    appendLog("Android network DNS changed; reconnecting the core.");
                    launchStart(true);
                } else if (resume) {
                    launchStart(false);
                }
            }

            @Override
            public void onLost(Network network) {
                synchronized (PROCESS_LOCK) {
                    if (isAlive(process)) {
                        runtimeState = "connecting";
                    }
                }
                updateNotification("Waiting for the network");
            }
        };
        try {
            connectivityManager.registerDefaultNetworkCallback(networkCallback);
        } catch (RuntimeException error) {
            appendLog("Unable to monitor Android network changes: " + error.getMessage());
            networkCallback = null;
        }
    }

    private void unregisterNetworkCallback() {
        if (connectivityManager == null || networkCallback == null) {
            return;
        }
        try {
            connectivityManager.unregisterNetworkCallback(networkCallback);
        } catch (RuntimeException ignored) {
            // The callback may already be unregistered while the process is shutting down.
        }
        networkCallback = null;
    }

    private void failAndStop(String message) {
        runtimeState = "error";
        setAutoStart(this, false);
        appendLog(message);
        updateNotification(message);
        stopSelf();
    }

    private static void stopProcessLocked() {
        Process active = process;
        process = null;
        if (active != null && isAlive(active)) {
            active.destroy();
            try {
                active.waitFor();
            } catch (InterruptedException error) {
                Thread.currentThread().interrupt();
            }
        }
    }

    private static boolean isAlive(Process candidate) {
        if (candidate == null) return false;
        try {
            candidate.exitValue();
            return false;
        } catch (IllegalThreadStateException ignored) {
            return true;
        }
    }

    private static String currentState(Context context) {
        synchronized (PROCESS_LOCK) {
            if (isAlive(process)) {
                if ("connected".equals(runtimeState)
                        || "connecting".equals(runtimeState)
                        || "restarting".equals(runtimeState)) {
                    return runtimeState;
                }
                return "connecting";
            }
            if (!isConfigured(context)) return "not installed";
            return runtimeState;
        }
    }

    private static boolean isConfigured(Context context) {
        return context.getSharedPreferences(PREFS, MODE_PRIVATE).getBoolean("configured", false)
                && new File(context.getFilesDir(), CONFIG_FILE).isFile();
    }

    private static boolean isBatteryOptimizationExempt(Context context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) {
            return true;
        }
        PowerManager powerManager = (PowerManager) context.getSystemService(Context.POWER_SERVICE);
        return powerManager != null && powerManager.isIgnoringBatteryOptimizations(context.getPackageName());
    }

    private static String binaryPath(Context context) {
        return context.getApplicationInfo().nativeLibraryDir + "/libfrpp.so";
    }

    private static void appendLog(String line) {
        if (line == null || line.trim().isEmpty()) return;
        line = ANSI_ESCAPE.matcher(line).replaceAll("");
        synchronized (LOGS) {
            LOGS.addLast(line);
            while (LOGS.size() > 200) {
                LOGS.removeFirst();
            }
        }
    }

    private static String dotenv(String value) {
        return "\"" + value
                .replace("\\", "\\\\")
                .replace("\r", "\\r")
                .replace("\n", "\\n")
                .replace("\"", "\\\"") + "\"";
    }

    private static String joinStrings(Iterable<String> values, String delimiter) {
        StringBuilder builder = new StringBuilder();
        for (String value : values) {
            if (builder.length() > 0) {
                builder.append(delimiter);
            }
            builder.append(value);
        }
        return builder.toString();
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "frp-manager connection",
                    NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription("Persistent client tunnel connection");
            getSystemService(NotificationManager.class).createNotificationChannel(channel);
        }
    }

    private Notification notification(String text) {
        Intent openIntent = new Intent(this, MainActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
                this,
                0,
                openIntent,
                PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT
        );
        return new NotificationCompat.Builder(this, CHANNEL_ID)
                .setSmallIcon(R.mipmap.ic_launcher)
                .setContentTitle("frp-manager Client")
                .setContentText(text)
                .setContentIntent(pendingIntent)
                .setOngoing(true)
                .setOnlyAlertOnce(true)
                .build();
    }

    private void updateNotification(String text) {
        getSystemService(NotificationManager.class).notify(NOTIFICATION_ID, notification(text));
    }
}
