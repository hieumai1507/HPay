// Demo data. Only ever loaded through a dynamic import guarded by
// `import.meta.env.DEV && MOCKS_ENABLED` (VITE_USE_MOCKS=true); it must never
// be reachable from a production build.
import type { ReconItem, ReconRun } from '@/queries/useReconQueries';

export const MOCK_RECON_RUN: ReconRun = {
  id: 'run-2026-07-07',
  date: '2026-07-07',
  totalCount: 2173,
  matchedCount: 2170,
  mismatchedCount: 3,
  runAt: '2026-07-08T00:15:00Z',
  durationMs: 4200,
  aiSummary:
    'Cả 3 bút toán lệch đều thuộc nhóm missing_external và phát sinh trong khung 23:58–00:02. Khả năng cao là lệch cutoff ngày giữa HPay và sao kê ngân hàng — giao dịch ghi nhận ngày 07/07 phía ví nhưng rơi sang ngày 08/07 phía bank. Đề xuất: đối chiếu lại với sao kê ngày 08/07 trước khi tạo case điều tra.',
  reportUrl: '/v1/files/recon/2026-07-07.xlsx?exp=1700000000&sig=mock',
};

export const MOCK_RECON_ITEMS: ReconItem[] = [
  { id: 'ri-1', status: 'missing_external', internalRef: 'MP26070799871', externalRef: null, internalAmount: 5000000, externalAmount: null, timestamp: '23:58:41', suggestedAction: 'Đối chiếu 08/07 →' },
  { id: 'ri-2', status: 'missing_external', internalRef: 'MP26070799902', externalRef: null, internalAmount: 1200000, externalAmount: null, timestamp: '23:59:12', suggestedAction: 'Đối chiếu 08/07 →' },
  { id: 'ri-3', status: 'missing_external', internalRef: 'MP26070799911', externalRef: null, internalAmount: 750000, externalAmount: null, timestamp: '00:01:03', suggestedAction: 'Đối chiếu 08/07 →' },
  { id: 'ri-4', status: 'matched', internalRef: 'MP26070799870', externalRef: 'VCB88213', internalAmount: 2000000, externalAmount: 2000000, timestamp: '23:55:18', suggestedAction: '—' },
];
