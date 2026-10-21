'use client'

import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { UpgradeFrppRequest } from '@/lib/pb/api_client'
import { upgradeFrpp } from '@/api/client'
import { upgradeServer } from '@/api/server'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export interface ClientUpgradeDialogProps {
 open: boolean
 onOpenChange: (open: boolean) => void
 clientId: string
 kind?: 'client' | 'server'
 defaultVersion?: string
 onDispatched?: () => void
}

export function ClientUpgradeDialog({open,onOpenChange,clientId,kind='client',defaultVersion='latest',onDispatched}: ClientUpgradeDialogProps) {
 const { t } = useTranslation()
 const [version,setVersion] = useState(defaultVersion)
 useEffect(() => { if(open) setVersion(defaultVersion) },[open,defaultVersion,clientId])
 const mutation = useMutation({
  mutationFn: kind === 'server' ? upgradeServer : upgradeFrpp,
  onSuccess: () => {
   toast.success(t('node_upgrade.accepted'))
   onOpenChange(false)
   onDispatched?.()
  },
  onError: (error: Error) => toast.error(t('node_upgrade.failed'),{description:error.message}),
 })
 return <Dialog open={open} onOpenChange={onOpenChange}>
  <DialogContent>
   <DialogHeader>
    <DialogTitle>{t('node_upgrade.title')}</DialogTitle>
    <DialogDescription>{t('node_upgrade.description',{id:clientId})}</DialogDescription>
   </DialogHeader>
   <div className="grid gap-3 py-2">
    <Label htmlFor="core-upgrade-version">{t('node_upgrade.version')}</Label>
    <Input id="core-upgrade-version" value={version} onChange={e=>setVersion(e.target.value)} placeholder={defaultVersion} disabled={mutation.isPending}/>
    <p className="text-sm text-muted-foreground">{t('node_upgrade.details')}</p>
    {kind==='server' && <p className="text-sm text-muted-foreground">{t('node_upgrade.legacy_server')}</p>}
   </div>
   <DialogFooter>
    <Button variant="outline" disabled={mutation.isPending} onClick={()=>onOpenChange(false)}>{t('common.cancel')}</Button>
    <Button disabled={mutation.isPending} onClick={()=>mutation.mutate(UpgradeFrppRequest.create({clientIds:[clientId],version:version.trim() || defaultVersion,backup:true,restartService:true}))}>
     {mutation.isPending ? t('common.loading') : t('node_upgrade.confirm')}
    </Button>
   </DialogFooter>
  </DialogContent>
 </Dialog>
}
