// Demo data. Only loaded via a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true).
export const MOCK_KPIS = [
  { label: 'Người dùng mới (hôm nay)', value: '128', delta: '+12%' },
  { label: 'Giao dịch (24h)', value: '2.431', delta: '+5%' },
  { label: 'Volume (24h)', value: '4,82 tỷ ₫', delta: '+9%' },
  { label: 'Tỉ lệ thất bại', value: '0,4%', delta: '-0,1đ' },
];

export const MOCK_VOLUME = [
  12, 8, 5, 3, 2, 4, 9, 18, 26, 31, 28, 35, 42, 38, 33, 36, 41, 47, 52, 44, 38, 29, 21, 15,
];
