import createClient from 'openapi-fetch'
import type { components, paths } from './schema.gen'

export type Health = components['schemas']['Health']

// Typed client for openapi.yaml. The API shares the PWA's origin, so there is
// no CORS and the session cookie travels on its own.
export const api = createClient<paths>({ baseUrl: '/api/v1' })
