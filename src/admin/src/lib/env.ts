/**
 * Mock data policy: mock/demo data may only be used in a Vite dev server AND
 * when VITE_USE_MOCKS=true is set explicitly. A production build can never
 * serve mock data: import.meta.env.DEV is statically false there.
 */
export function isMockMode(dev: boolean | undefined, flag: string | undefined): boolean {
  return dev === true && flag === 'true';
}

export const MOCKS_ENABLED: boolean = isMockMode(import.meta.env.DEV, import.meta.env.VITE_USE_MOCKS);