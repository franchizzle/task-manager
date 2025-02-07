import { createRoot } from 'react-dom/client'
import * as Sentry from '@sentry/react'
import App from '../App'
import { isDevelopmentMode } from './environment'
import './jank-mode'

if (!isDevelopmentMode) {
    Sentry.init({
        dsn: 'https://9181589921b12cd7b59f760de07f8abb@o4508777211494400.ingest.us.sentry.io/4508777218179073',
        environment: process.env.NODE_ENV,

        // This sets the sample rate to be 10%. You may want this to be 100% while
        // in development and sample at a lower rate in production
        replaysSessionSampleRate: 1.0,
        // If the entire session is not sampled, use the below sample rate to sample
        // sessions when an error occurs.
        replaysOnErrorSampleRate: 1.0,

        integrations: [new Sentry.Replay({ blockAllMedia: false })],
    })
}
const root = createRoot(document.getElementById('root') as Element)
root.render(<App />)
