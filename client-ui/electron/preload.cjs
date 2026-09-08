const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('frpManager', {
  getProfiles: () => ipcRenderer.invoke('profiles:list'),
  saveProfile: (profile) => ipcRenderer.invoke('profiles:save', profile),
  deleteProfile: (id) => ipcRenderer.invoke('profiles:delete', id),
  getRuntimeStatus: () => ipcRenderer.invoke('runtime:status'),
  chooseBinary: () => ipcRenderer.invoke('runtime:chooseBinary'),
  runAction: (action, profile) => ipcRenderer.invoke('runtime:action', action, profile),
  openDataDir: () => ipcRenderer.invoke('runtime:openDataDir'),
})
