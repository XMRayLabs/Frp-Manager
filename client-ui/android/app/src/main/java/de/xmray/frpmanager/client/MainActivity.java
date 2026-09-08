package de.xmray.frpmanager.client;

import android.os.Bundle;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(FrpManagerPlugin.class);
        super.onCreate(savedInstanceState);
    }
}
