import { LanguageSwitcher } from '@/components/language-switcher'
import { REPOSITORY_URL } from '@/lib/consts'
import { ArrowUpRight } from 'lucide-react'
import Link from 'next/link'
import Image from 'next/image'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

export function AuthShell({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string
  subtitle: string
  children: ReactNode
  footer: ReactNode
}) {
  const { t } = useTranslation()

  return (
    <main className="grid min-h-screen w-full min-w-0 bg-background lg:grid-cols-[minmax(320px,0.8fr)_minmax(480px,1.2fr)]">
      <aside className="hidden flex-col justify-between border-r bg-[#17191c] p-10 text-white lg:flex">
        <Link href="/" className="flex items-center gap-3">
          <Image src="/frp-manager-icon.svg" alt="" width={44} height={44} unoptimized className="size-11 rounded-md" />
          <div>
            <div className="text-base font-semibold">frp-manager</div>
            <div className="text-xs text-white/55">{t('app.subtitle')}</div>
          </div>
        </Link>
        <div className="max-w-md">
          <div className="mb-4 h-1 w-12 rounded-full bg-cyan-400" />
          <p className="text-xl font-medium leading-relaxed text-white/90">{t('app.description')}</p>
          <a
            className="mt-6 inline-flex items-center gap-2 text-sm text-cyan-300 hover:text-cyan-200"
            href={REPOSITORY_URL}
            target="_blank"
            rel="noopener noreferrer"
          >
            {t('app.github.repo')}
            <ArrowUpRight className="size-4" />
          </a>
        </div>
        <div className="text-xs text-white/40">FRP orchestration console</div>
      </aside>

      <section className="relative flex min-h-screen items-center justify-center px-5 py-16">
        <div className="absolute right-5 top-5">
          <LanguageSwitcher />
        </div>
        <div className="w-full max-w-sm">
          <Link href="/" className="mb-10 flex items-center gap-3 lg:hidden">
            <Image src="/frp-manager-icon.svg" alt="" width={40} height={40} unoptimized className="size-10 rounded-md" />
            <span className="font-semibold">frp-manager</span>
          </Link>
          <div className="mb-7">
            <h1 className="text-2xl font-semibold">{title}</h1>
            <p className="mt-2 text-sm text-muted-foreground">{subtitle}</p>
          </div>
          {children}
          <div className="mt-6 text-center text-sm text-muted-foreground">{footer}</div>
        </div>
      </section>
    </main>
  )
}
