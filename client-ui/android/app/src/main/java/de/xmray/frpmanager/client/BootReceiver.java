package de.xmray.frpmanager.client;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent.getAction();
        boolean shouldRestore = Intent.ACTION_BOOT_COMPLETED.equals(action)
                || Intent.ACTION_MY_PACKAGE_REPLACED.equals(action);
        if (shouldRestore && FrpManagerService.shouldAutoStart(context)) {
            FrpManagerService.start(context, false);
        }
    }
}
