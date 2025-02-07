const ENV = {
    dev: {
        REACT_APP_API_BASE_URL: 'http://localhost:8080',
        REACT_APP_FRONTEND_BASE_URL: 'http://localhost:3000',
        REACT_APP_NOTES_BASE_URL: 'http://localhost:3000',
        REACT_APP_TASK_BASE_URL: 'http://localhost:8080',
        REACT_APP_TRY_BASE_URL: 'http://localhost:3000',
        REACT_APP_TRY_SIGN_UP_URL: 'http://localhost:3000',
        COOKIE_DOMAIN: '.localhost',
    },
    prod: {
        REACT_APP_API_BASE_URL: 'https://general-task-manager-760b0b54c766.herokuapp.com',
        REACT_APP_FRONTEND_BASE_URL: 'https://resonant-kelpie-404a42.netlify.app',
        REACT_APP_NOTES_BASE_URL: 'https://notes.resonant-kelpie-404a42.netlify.app',
        REACT_APP_TASK_BASE_URL: 'https://share.resonant-kelpie-404a42.netlify.app',
        REACT_APP_TRY_BASE_URL: 'https://try.resonant-kelpie-404a42.netlify.app',
        REACT_APP_TRY_SIGN_UP_URL: 'https://try.gresonant-kelpie-404a42.netlify.app/sign-up',
        COOKIE_DOMAIN: '.resonant-kelpie-404a42.netlify.app',
    },
}

export const isDevelopmentMode: boolean = !process.env.NODE_ENV || process.env.NODE_ENV === 'development'

const getEnvVars = () => {
    return isDevelopmentMode ? ENV.dev : ENV.prod
}

export default getEnvVars
