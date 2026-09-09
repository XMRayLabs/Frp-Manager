"use client"

import i18n from '@/lib/i18n'
import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { initClient, listClient } from '@/api/client'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { RespCode } from '@/lib/pb/common'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { IsIDValid } from '@/lib/consts'
import { useStore } from '@nanostores/react'
import { $userInfo } from '@/store/user'

export const CreateClientDialog = ({refetchTrigger, triggerLabel}: {refetchTrigger?: (randStr: string) => void; triggerLabel?: string}) => {
  const { t, i18n } = useTranslation()
  const user = useStore($userInfo)
  const zh = i18n.language.startsWith('zh')
  const [clientID, setClientID] = useState<string | undefined>()
  const newClient = useMutation({
    mutationFn: initClient,
  })

  const handleNewClient = async () => {
    toast(t('client.create.submitting'))
    try {
      let resp = await newClient.mutateAsync({ clientId: clientID })
      if (resp.status?.code !== RespCode.SUCCESS) {
        toast(t('client.create.error'),{
          description: resp.status?.message
        })
        return
      }
      toast(t('client.create.success'))
      refetchTrigger && refetchTrigger(JSON.stringify(Math.random()))
    } catch (error) {
      toast(t('client.create.error'), {
        description: JSON.stringify(error)
      })
    }
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline">
          {triggerLabel ?? t('client.create.button')}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('client.create.title')}</DialogTitle>
          <DialogDescription>{t('client.create.description')}</DialogDescription>
        </DialogHeader>

        <Label htmlFor="new-client-name">{zh ? '客户端名称' : 'Client name'}</Label>
        <Input id="new-client-name" placeholder={zh ? '例如 home、office' : 'e.g. home, office'} value={clientID || ''} onChange={(e) => setClientID(e.target.value)} />
        <p className="text-sm text-muted-foreground">{zh ? '只填写名称（字母、数字、下划线或短横线），用户前缀自动添加。' : 'Enter a name using letters, digits, underscores or hyphens. The user prefix is automatic.'}</p>
        {user?.userName && clientID && <p className="break-all font-mono text-xs">{user.userName}.c.{clientID}</p>
        }
        <DialogFooter>
          <Button onClick={handleNewClient}
          disabled={!IsIDValid(clientID)}
          className='w-full'>{t('client.create.submit')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
