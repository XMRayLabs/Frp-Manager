import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'de.xmray.frpmanager.client',
  appName: 'frp-manager Client',
  webDir: 'dist',
  server: {
    androidScheme: 'https',
  },
}

export default config
