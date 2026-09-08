import { Providers } from '@/components/providers'
import '@/styles/globals.css'
import '@xyflow/react/dist/style.css'
import type { AppProps } from 'next/app'
import { I18nextProvider } from 'react-i18next'
import i18n from '@/lib/i18n'
import Head from 'next/head'

export default function App({ Component, pageProps }: AppProps) {
  return (
    <>
      <Head>
        <title>frp-manager</title>
        <meta name="description" content="FRP orchestration and tunnel management console" />
        <meta name="theme-color" content="#17191c" />
        <link rel="icon" href="/frp-manager-icon.svg" />
      </Head>
      <I18nextProvider i18n={i18n}>
        <Providers>
          <Component {...pageProps} />
        </Providers>
      </I18nextProvider>
    </>
  )
}
