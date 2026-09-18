import React, { useState } from 'react';
import { Donation } from '../types';
import { donationAPI } from '../api';

interface RefundModalProps {
  donation: Donation;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const RefundModal: React.FC<RefundModalProps> = ({ donation, isOpen, onClose, onSuccess }) => {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reason.trim()) {
      alert('请填写退款原因');
      return;
    }
    setLoading(true);
    try {
      await donationAPI.applyRefund(donation.id, { reason: reason.trim() });
      alert('退款申请已提交，等待平台审核');
      setReason('');
      onSuccess();
      onClose();
    } catch (error: any) {
      const status = error.response?.status;
      const msg = error.response?.data?.message || '申请失败，请稍后重试';
      alert(status === 409 ? msg : msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-2xl p-8 max-w-md w-full mx-4">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-2xl font-bold text-gray-900">申请退款</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="mb-6 p-4 bg-amber-50 border border-amber-200 rounded-lg text-sm text-amber-800">
          捐赠成功后 24 小时内可申请该笔捐赠的全额退款，提交后在审核期间电子凭证将暂时冻结。
        </div>

        <div className="mb-6 p-4 bg-gray-50 rounded-lg space-y-1 text-sm">
          <p className="text-gray-600">退款项目</p>
          <p className="font-semibold text-gray-900">{donation.project?.title}</p>
          <p className="text-primary-600 font-semibold">退款金额：¥{donation.amount.toLocaleString()}</p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-2">
              退款原因 <span className="text-red-500">*</span>
            </label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="请说明申请退款的原因（必填）"
              maxLength={255}
              className="w-full px-4 py-3 border border-gray-200 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent resize-none"
              rows={4}
            />
            <div className="text-right text-xs text-gray-400 mt-1">{reason.length}/255</div>
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

export default RefundModal;
