import { RefundStatus } from '../types';

export const refundStatusMeta: Record<RefundStatus, { label: string; className: string }> = {
  pending: { label: '审核中', className: 'bg-amber-100 text-amber-700' },
  approved: { label: '已批准', className: 'bg-green-100 text-green-700' },
  rejected: { label: '已驳回', className: 'bg-red-100 text-red-700' },
};

// 捐赠成功后 24 小时内可申请退款。
export const REFUND_WINDOW_MS = 24 * 60 * 60 * 1000;

// canApplyRefund 判断一笔成功捐赠当前是否可由捐赠人申请退款（仅时间窗口，状态另判）。
export const canApplyRefund = (createdAt: string, now: number = Date.now()): boolean => {
  const t = new Date(createdAt).getTime();
  return Number.isFinite(t) && now - t <= REFUND_WINDOW_MS;
};
