import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';

import { ApiError } from '@/api/errors';
import { toneForVerdict, useKYCQueue, useKYCReview, type KYCTab } from '@/queries';
import { useAdminStore } from '@/stores/admin';
import { useToastStore } from '@/stores/toast';
import { Badge, Button, Modal } from '@/ui';

function SignedImage({ src, alt, onExpired }: { src: string; alt: string; onExpired: () => void }) {
  const retried = useRef(false);
  return (
    <img
      src={src}
      alt={alt}
      onError={() => {
        if (!retried.current) {
          retried.current = true;
          onExpired();
        }
      }}
      className="w-full rounded-lg border border-slate-200 bg-slate-100 min-h-[180px] object-contain"
    />
  );
}

export default function KYCDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const tab = (params.get('tab') as KYCTab) ?? 'manual';
  const navigate = useNavigate();
  const toast = useToastStore((s) => s.push);
  const can = useAdminStore((s) => s.can);

  const { data, isPending, refetch } = useKYCQueue(tab);
  const review = useKYCReview();

  const [rejectOpen, setRejectOpen] = useState(false);
  const [reason, setReason] = useState('');

  const rows = useMemo(() => data?.rows ?? [], [data]);
  const idx = useMemo(() => rows.findIndex((r) => r.id === id), [rows, id]);
  const item = idx >= 0 ? rows[idx] : undefined;
  const canReview = can('kyc.review') && tab === 'manual';

  const goSibling = (dir: 1 | -1) => {
    if (rows.length === 0) return;
    const next = rows[idx + dir];
    if (next) navigate(`/kyc/${next.id}?tab=${tab}`, { replace: true });
  };

  const decide = (action: 'approve' | 'reject', reasonText?: string) => {
    if (!item || review.isPending) return;
    review.mutate(
      { id: item.id, action, reason: reasonText },
      {
        onSuccess: () => {
          toast(action === 'approve' ? 'Đã phê duyệt hồ sơ.' : 'Đã từ chối hồ sơ.', 'success');
          setRejectOpen(false);
          setReason('');
          const next = rows[idx + 1] ?? rows[idx - 1];
          if (next) navigate(`/kyc/${next.id}?tab=${tab}`, { replace: true });
          else navigate(`/kyc?tab=${tab}`);
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 409) {
            toast('Hồ sơ đã được xử lý ở nơi khác — danh sách vừa được cập nhật.', 'info');
            navigate(`/kyc?tab=${tab}`);
          } else {
            toast(err instanceof ApiError ? err.message : 'Có lỗi xảy ra.', 'error');
          }
        },
      },
    );
  };

  // Latest-handler ref so the key listener calls current closures without re-subscribing.
  const handlers = useRef({ goSibling, decide });
  useEffect(() => {
    handlers.current = { goSibling, decide };
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || rejectOpen) return;
      if (e.key === 'j' || e.key === 'J') {
        e.preventDefault();
        handlers.current.goSibling(1);
      }
      if (e.key === 'k' || e.key === 'K') {
        e.preventDefault();
        handlers.current.goSibling(-1);
      }
      if ((e.key === 'a' || e.key === 'A') && canReview) {
        e.preventDefault();
        handlers.current.decide('approve');
      }
      if ((e.key === 'r' || e.key === 'R') && canReview) {
        e.preventDefault();
        setRejectOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [rejectOpen, canReview]);

  if (isPending) return <div className="text-slate-500 p-6">Đang tải…</div>;
  if (!item)
    return (
      <div className="text-slate-500 p-6">
        Không tìm thấy hồ sơ trong tab này (có thể đã được xử lý).{' '}
        <a href={`/kyc?tab=${tab}`} className="text-brand font-semibold hover:underline">← Về danh sách</a>
      </div>
    );

  const reasons: string[] = Array.isArray(item.ai_reasons)
    ? item.ai_reasons.map(String)
    : item.ai_reasons
      ? [JSON.stringify(item.ai_reasons)]
      : [];

  return (
    <div className="flex flex-col gap-4">
      {/* Header */}
      <div className="flex justify-between items-center flex-wrap gap-3">
        <div className="flex items-center gap-2.5">
          <Button variant="ghost" onClick={() => navigate(`/kyc?tab=${tab}`)}>
            ← Danh sách
          </Button>
          <strong className="text-base font-bold text-slate-900">{item.full_name}</strong>
          <Badge tone={toneForVerdict(item.ai_verdict)}>{item.ai_verdict}</Badge>
        </div>
        <div className="text-xs text-slate-500">
          {idx + 1}/{rows.length} · <code className="font-mono bg-slate-100 px-1 rounded">J</code>/<code className="font-mono bg-slate-100 px-1 rounded">K</code> chuyển hồ sơ
          {canReview && (
            <>
              {' '}
              · <code className="font-mono bg-slate-100 px-1 rounded">A</code> duyệt · <code className="font-mono bg-slate-100 px-1 rounded">R</code> từ chối
            </>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Images */}
        <div className="grid grid-cols-2 gap-3">
          <div>
            <div className="text-xs font-semibold text-slate-700 mb-1.5">CCCD</div>
            <SignedImage src={item.id_card_url} alt="CCCD" onExpired={() => refetch()} />
          </div>
          <div>
            <div className="text-xs font-semibold text-slate-700 mb-1.5">Selfie</div>
            <SignedImage src={item.selfie_url} alt="Selfie" onExpired={() => refetch()} />
          </div>
        </div>

        {/* Info & Decision */}
        <div className="flex flex-col gap-3.5">
          <div className="bg-white border border-slate-200 rounded-xl p-4 shadow-xs">
            <strong className="text-sm font-bold text-slate-900">Thông tin đăng ký</strong>
            <table className="mt-2 text-xs w-full border-separate border-spacing-y-1">
              <tbody>
                <tr>
                  <td className="text-slate-500 pr-4 pb-1">Họ tên</td>
                  <td className="font-semibold text-slate-900">{item.full_name}</td>
                </tr>
                <tr>
                  <td className="text-slate-500 pr-4 pb-1">SĐT</td>
                  <td>
                    <code className="font-mono text-slate-700">{item.phone}</code>
                  </td>
                </tr>
                <tr>
                  <td className="text-slate-500 pr-4 pb-1">Nộp lúc</td>
                  <td className="text-slate-700">{new Date(item.created_at).toLocaleString('vi-VN')}</td>
                </tr>
                <tr>
                  <td className="text-slate-500 pr-4">AI score</td>
                  <td className="font-bold text-slate-900">{item.ai_score}/100</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="bg-white border border-slate-200 rounded-xl p-4 shadow-xs">
            <strong className="text-sm font-bold text-slate-900">AI phân tích</strong>
            {reasons.length === 0 ? (
              <p className="text-slate-500 text-xs mt-1.5">Không có ghi chú từ AI.</p>
            ) : (
              <ul className="mt-1.5 pl-4 list-disc text-xs text-slate-700 space-y-1">
                {reasons.map((r, i) => (
                  <li key={i}>{r}</li>
                ))}
              </ul>
            )}
          </div>

          {canReview && (
            <div className="flex gap-2.5">
              <Button
                className="flex-1"
                loading={review.isPending}
                onClick={() => decide('approve')}
              >
                ✓ Phê duyệt (A)
              </Button>
              <Button variant="danger" className="flex-1" onClick={() => setRejectOpen(true)}>
                ✕ Từ chối (R)
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Reject Modal */}
      <Modal
        open={rejectOpen}
        title="Từ chối hồ sơ KYC"
        onClose={() => setRejectOpen(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setRejectOpen(false)}>
              Hủy
            </Button>
            <Button
              variant="danger"
              disabled={reason.trim().length < 10}
              loading={review.isPending}
              onClick={() => decide('reject', reason.trim())}
            >
              Xác nhận từ chối
            </Button>
          </>
        }
      >
        <p className="mb-2.5 text-xs text-slate-700">
          Lý do bắt buộc (≥ 10 ký tự) — được ghi vào audit log.
        </p>
        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
          autoFocus
          placeholder="Ví dụ: Ảnh CCCD mờ, không khớp khuôn mặt selfie…"
          className="w-full p-2.5 rounded-lg border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-rose-200 focus:border-rose-500"
        />
        {reason.length > 0 && reason.trim().length < 10 && (
          <div className="text-rose-600 text-xs mt-1">
            Cần tối thiểu 10 ký tự ({reason.trim().length}/10).
          </div>
        )}
      </Modal>
    </div>
  );
}
