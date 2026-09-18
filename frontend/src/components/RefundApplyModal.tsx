import React, { useState } from 'react';
import { Donation } from '../types';
import { donationAPI } from '../api';

interface RefundApplyModalProps {
  donation: Donation;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const RefundApplyModal: React.FC<RefundApplyModalProps> = ({ donation, isOpen, onClose, onSuccess }) => {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  // 捐赠成功后 24 小时内可申请全额退款。
  const deadline = new Date(new Date(donation.createdAt).getTime() + 24 * 60 * 60 * 1000);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reason.trim()) {
      alert('请填写退款原因');
      return;
    }
    setLoading(true);
    try {
      await donationAPI.applyRefund(donation.id, { reason: reason.trim() });
      alert('退款申请已提交，等待管理员审核。审核期间凭证将暂时冻结。');
      setReason('');
      onSuccess();
      onClose();
    } catch (error: any) {
      alert(error.response?.data?.message || '退款申请失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-2xl p-8 max-w-md w-full mx-4">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-2xl font-bold text-gray-900">申请全额退款</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="mb-6 p-4 bg-gray-50 rounded-lg space-y-1 text-sm">
          <p className="text-gray-600">
            捐赠项目：<span className="font-semibold text-gray-900">{donation.project?.title}</span>
          </p>
          <p className="text-gray-600">
            退款金额：<span className="font-semibold text-primary-600">¥{donation.amount.toLocaleString()}</span>（全额）
          </p>
          <p className="text-gray-500 text-xs">申请截止：{deadline.toLocaleString()}（捐赠成功后 24 小时内）</p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-2">退款原因</label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="请说明申请退款的原因（必填，500 字以内）"
              maxLength={500}
              className="w-full px-4 py-3 border border-gray-200 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent resize-none"
              rows={4}
            />
            <div className="text-right text-xs text-gray-400 mt-1">{reason.length}/500</div>
          </div>

          <div className="bg-amber-50 text-amber-700 text-xs rounded-lg p-3 mb-4">
            提交后该笔捐赠凭证将在审核期间冻结展示；同一笔捐赠只能提交一次退款申请。
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-primary-600 text-white py-3 rounded-lg font-semibold hover:bg-primary-700 disabled:opacity-50"
          >
            {loading ? '提交中...' : '确认提交退款申请'}
          </button>
        </form>
      </div>
    </div>
  );
};

export default RefundApplyModal;
