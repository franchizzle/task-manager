import { lazy } from 'react'
import ReactGA from 'react-ga4'
import { Helmet } from 'react-helmet'
import { QueryClient, QueryClientProvider } from 'react-query'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import AuthenticatedRoutes from './src/AuthenticatedRoutes'
import SharedNoteView from './src/components/notes/SharedNoteView'
import GoogleAuth from './src/components/screens/GoogleAuthScreen'
// import LandingScreen from './src/components/screens/LandingScreen'
import SharedTaskView from './src/components/views/SharedTaskView'
import {
    GOOGLE_AUTH_ROUTE,
    NOTE_ROUTE,
    PRIVACY_POLICY_ROUTE,
    SHAREABLE_TASK_ROUTE,
    TERMS_OF_SERVICE_ROUTE,
} from './src/constants'
import { GlobalStyle } from './src/styles'
import { CompanyPolicyPages } from './src/utils/enums'

const GA_TRACKING_ID = 'G-GLHZBNMPN9'
ReactGA.initialize(GA_TRACKING_ID, {
    gaOptions: {
        siteSpeedSampleRate: 100,
    },
})

const CompanyPolicyView = lazy(() => import('./src/components/views/CompanyPolicyView'))

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            staleTime: Infinity,
            cacheTime: Infinity,
            refetchIntervalInBackground: true,
        },
    },
})

const App = () => {
    return (
        <QueryClientProvider client={queryClient}>
            <Helmet>
                <meta charSet="utf-8" />
                <title>General Task (beta)</title>
                <link rel="icon" href="/images/favicon.png" />
                <script src="https://kit.fontawesome.com/af07b49f89.js" crossOrigin="anonymous"></script>
                <base target="_blank" />
                <meta
                    content="Find focus like never before with the best free productivity tool on the market!"
                    name="description"
                />
                <meta
                    content="General Task (beta) — Free productivity tools for builders like you"
                    property="og:title"
                />
                <meta
                    content="Find focus like never before with the best free productivity tool on the market!"
                    property="og:description"
                />
                <meta
                    content="General Task (beta) — Free productivity tools for builders like you"
                    property="twitter:title"
                />
                <meta
                    content="Find focus like never before with the best free productivity tool on the market!"
                    property="twitter:description"
                />
                <meta property="og:type" content="website" />
                <meta property="og:image" content="/images/futureman.png" />
                <meta content="summary_large_image" name="twitter:card" />
            </Helmet>
            <BrowserRouter>
                <GlobalStyle />

                <Routes>
                    <Route index element={<GoogleAuth />} />
                    <Route
                        path={TERMS_OF_SERVICE_ROUTE}
                        element={<CompanyPolicyView page={CompanyPolicyPages.TermsOfService} />}
                    />
                    <Route
                        path={PRIVACY_POLICY_ROUTE}
                        element={<CompanyPolicyView page={CompanyPolicyPages.PrivacyPolicy} />}
                    />
                    <Route path={NOTE_ROUTE} element={<SharedNoteView />}>
                        <Route path=":noteId" element={<SharedNoteView />} />
                    </Route>
                    <Route path={GOOGLE_AUTH_ROUTE} element={<GoogleAuth />} />
                    <Route path={SHAREABLE_TASK_ROUTE} element={<SharedTaskView />}>
                        <Route path=":taskId" element={<SharedTaskView />} />
                    </Route>
                    <Route path="*" element={<AuthenticatedRoutes />} />
                    <Route path="*" element={<Navigate to="/" />} />
                </Routes>
            </BrowserRouter>
        </QueryClientProvider>
    )
}

export default App
