import { useQueryClient } from '@tanstack/react-query'
import { ZodStringSchema } from '@/lib/consts'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import { Form, FormControl, FormField, FormItem, FormMessage, FormLabel } from '@/components/ui/form'
import { Input } from './ui/input'
import { login } from '@/api/auth'
import { Button } from './ui/button'

import { ExclamationTriangleIcon } from '@radix-ui/react-icons'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { useState } from 'react'
import { RespCode } from '@/lib/pb/common'
import { useRouter } from 'next/router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'

export const LoginSchema = z.object({
  username: ZodStringSchema,
  password: ZodStringSchema,
})

export function LoginComponent() {
  const { t } = useTranslation()
  const form = useForm<z.infer<typeof LoginSchema>>({
    resolver: zodResolver(LoginSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })
  const router = useRouter()
  const queryClient = useQueryClient()

  const [loginAlert, setLoginAlert] = useState(false)

  const onSubmit = async (values: z.infer<typeof LoginSchema>) => {
    toast(t('auth.loggingIn'))
    try {
      const res = await login({ ...values })
      if (res.status?.code === RespCode.SUCCESS) {
        queryClient.clear()
        toast(t('auth.loginSuccess'))
        await router.replace('/')
        setLoginAlert(false)
      } else {
        toast(t('auth.loginFailed'), {
          description: res.status?.message,
        })
        setLoginAlert(true)
      }
    } catch (e) {
      toast(t('auth.loginFailed'), {
        description: (e as Error).message,
      })
      setLoginAlert(true)
    }
  }

  return (
    <div className="w-full flex flex-col gap-6">
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
          <FormField
            control={form.control}
            name="username"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('auth.usernamePlaceholder')}</FormLabel>
                <FormControl>
                  <Input autoComplete="username" type="text" placeholder={t('auth.usernamePlaceholder')} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="password"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('auth.password')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete="current-password"
                    type="password"
                    placeholder={t('auth.passwordPlaceholder')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          {loginAlert && (
            <Alert variant="destructive">
              <ExclamationTriangleIcon className="h-4 w-4" />
              <AlertTitle>{t('auth.error')}</AlertTitle>
              <AlertDescription>{t('auth.loginFailed')}</AlertDescription>
            </Alert>
          )}
          <Button className="w-full" type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting && <Loader2 className="mr-2 size-4 animate-spin" />}
            {t('common.login')}
          </Button>
        </form>
      </Form>
    </div>
  )
}
