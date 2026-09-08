package de.xmray.frpmanager.client;

import android.content.Intent;
import android.net.Uri;
import android.provider.Settings;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

import java.util.Locale;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

@CapacitorPlugin(name = "FrpManager")
public class FrpManagerPlugin extends Plugin {
    private static final Set<String> PROFILE_ACTIONS = new HashSet<>(
            Arrays.asList("apply", "install", "start", "restart", "run")
    );
    private static final Set<String> ACTIONS = new HashSet<>(
            Arrays.asList("apply", "install", "start", "stop", "restart", "run", "uninstall")
    );
    private static final Set<String> SECURE_RPC_SCHEMES = new HashSet<>(Arrays.asList("grpc", "wss"));

    @PluginMethod
    public void getRuntimeStatus(PluginCall call) {
        call.resolve(FrpManagerService.runtimeStatus(getContext()));
    }

    @PluginMethod
    public void openPowerSettings(PluginCall call) {
        try {
            Intent intent = new Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS);
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            getContext().startActivity(intent);
            call.resolve();
        } catch (Exception error) {
            call.reject("Unable to open Android battery settings.", error);
        }
    }

    @PluginMethod
    public void runAction(PluginCall call) {
        String action = call.getString("action", "");
        JSObject profile = call.getObject("profile");
        try {
            if (!ACTIONS.contains(action)) {
                throw new IllegalArgumentException("Unsupported action: " + action);
            }
            if (PROFILE_ACTIONS.contains(action)) {
                validateProfile(profile);
                FrpManagerService.saveProfile(getContext(), profile);
            }

            String message;
            switch (action) {
                case "install":
                    FrpManagerService.setAutoStart(getContext(), false);
                    message = "Client configuration installed.";
                    break;
                case "apply":
                case "start":
                case "run":
                    FrpManagerService.setAutoStart(getContext(), true);
                    FrpManagerService.start(getContext(), false);
                    message = "Client start requested.";
                    break;
                case "restart":
                    FrpManagerService.setAutoStart(getContext(), true);
                    FrpManagerService.start(getContext(), true);
                    message = "Client restart requested.";
                    break;
                case "stop":
                    FrpManagerService.setAutoStart(getContext(), false);
                    FrpManagerService.stop(getContext());
                    message = "Client stopped.";
                    break;
                case "uninstall":
                    FrpManagerService.setAutoStart(getContext(), false);
                    FrpManagerService.stop(getContext());
                    FrpManagerService.clearProfile(getContext());
                    message = "Client configuration removed.";
                    break;
                default:
                    throw new IllegalArgumentException("Unsupported action: " + action);
            }

            JSObject result = new JSObject();
            result.put("ok", true);
            result.put("message", message);
            result.put("output", FrpManagerService.recentLogs());
            result.put("status", FrpManagerService.runtimeStatus(getContext()));
            call.resolve(result);
        } catch (Exception error) {
            JSObject result = new JSObject();
            result.put("ok", false);
            result.put("message", error.getMessage());
            result.put("output", error.getMessage());
            result.put("status", FrpManagerService.runtimeStatus(getContext()));
            call.resolve(result);
        }
    }

    private static void validateProfile(JSObject profile) {
        if (profile == null) {
            throw new IllegalArgumentException("Client profile is required.");
        }
        for (String key : new String[]{"apiUrl", "rpcUrl"}) {
            if (profile.optString(key, "").trim().isEmpty()) {
                throw new IllegalArgumentException("Missing " + key + ".");
            }
        }

        if (profile.optString("joinToken").isEmpty() && (profile.optString("clientId").isEmpty() || profile.optString("secret").isEmpty())) {
            throw new IllegalArgumentException("Provide an enrollment token or client ID and secret.");
        }
        Uri api = parseEndpoint(profile.optString("apiUrl"), "API URL");
        Uri rpc = parseEndpoint(profile.optString("rpcUrl"), "RPC URL");
        if (api.getUserInfo() != null || rpc.getUserInfo() != null) {
            throw new IllegalArgumentException("Endpoint URLs must not contain embedded credentials.");
        }
        if (profile.optBoolean("allowInsecure", false)) {
            return;
        }

        if (!isLoopback(api.getHost()) && !"https".equals(lower(api.getScheme()))) {
            throw new IllegalArgumentException("Remote API URL must use HTTPS.");
        }
        String rpcScheme = lower(rpc.getScheme());
        if (!isLoopback(rpc.getHost()) && !SECURE_RPC_SCHEMES.contains(rpcScheme)) {
            throw new IllegalArgumentException("Remote RPC URL must use grpc:// or wss://.");
        }
    }

    private static Uri parseEndpoint(String value, String label) {
        Uri uri = Uri.parse(value);
        if (uri.getScheme() == null || uri.getHost() == null) {
            throw new IllegalArgumentException(label + " is invalid.");
        }
        return uri;
    }

    private static boolean isLoopback(String host) {
        String normalized = lower(host);
        return Arrays.asList("localhost", "127.0.0.1", "::1", "[::1]").contains(normalized);
    }

    private static String lower(String value) {
        return value == null ? "" : value.toLowerCase(Locale.ROOT);
    }
}
