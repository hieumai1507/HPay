// Demo data. Only loaded via a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true).
export const MOCK_KPIS = [
  { label: 'New users (today)', value: '128', delta: '+12%' },
  { label: 'Transactions (24h)', value: '2,431', delta: '+5%' },
  { label: 'Volume (24h)', value: '4,820,000,000 ₫', delta: '+9%' },
  { label: 'Failure rate', value: '0.4%', delta: '-0.1 pt' },
];

export const MOCK_VOLUME = [
  12, 8, 5, 3, 2, 4, 9, 18, 26, 31, 28, 35, 42, 38, 33, 36, 41, 47, 52, 44, 38, 29, 21, 15,
];
