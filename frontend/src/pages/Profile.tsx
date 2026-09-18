import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { donationAPI, refundAPI } from '../api';
import { Donation, RefundApplication, RefundStatus } from '../types';
import RefundApplyModal from '../components/RefundApplyModal';

const REFUND_WINDOW_MS = 24 * 60 * 60 * 1000;

const refundStatusMeta: Record<RefundStatus, { label: string; cls: string }> = {
  pending: { label: '审核中', cls: 'bg-amber-100 text-amber-700' },
  approved: { label: '已批准', cls: 'bg-green-100 text-green-700' },
  rejected: { label: '已驳回', cls: 'bg-red-100 text-red-700' },
};

const Profile = () => {
  const { user, logout } = useAuth();
  const [donations, setDonations] = useState<Donation[]>([]);
  const [refunds, setRefunds] = useState<RefundApplication[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'info' | 'donations' | 'refunds'>('info');
  const [refundTarget, setRefundTarget] = useState<Donation | null>(null);

  useEffect(() => {
    if (user) loadData();
  }, [user]);

  const loadData = async () => {
    try {
      const [donRes, refundRes] = await Promise.all([
        donationAPI.getMyDonations({ limit: 20 }),
        refundAPI.getMyRefunds({ limit: 20 }),
      ]);
      setDonations(donRes.data.donations);
      setRefunds(refundRes.data.refunds || []);
    } catch (error) {
      console.error('加载数据失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const viewCertificate = async (donationId: string) => {
    try {
      const response = await donationAPI.getCertificate(donationId);
      const cert = response.data.certificate;
      alert(`电子凭证\n\n凭证编号：${cert.certificateNo}\n捐赠金额：¥${cert.amount}\n项目：${cert.projectTitle}\n捐赠人：${cert.donorName}\n捐赠时间：${new Date(cert.createdAt).toLocaleString()}`);
    } catch (error: any) {
      alert(error.response?.data?.message || '获取凭证失败');
    }
  };

  // 是否仍在 24 小时可退款窗口内。
  const withinRefundWindow = (d: Donation) =>
    Date.now() - new Date(d.createdAt).getTime() < REFUND_WINDOW_MS;

  if (!user) {
    return <Navigate to="/login" />;
  }

  const roleMap: Record<string, string> = {
    user: '个人用户',
    org: '公益组织',
    admin: '管理员',
  };

  const renderRefundBadge = (status?: RefundStatus) => {
    if (!status) return null;
    const meta = refundStatusMeta[status];
    return (
      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${meta.cls}`}>{meta.label}</span>
    );
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="bg-white rounded-2xl shadow-sm overflow-hidden mb-6">
        <div className="bg-gradient-to-r from-primary-500 to-primary-600 h-32" />
        <div className="px-8 pb-8">
          <div className="flex items-end gap-6 -mt-12 mb-6">
            <div className="w-24 h-24 bg-white rounded-full border-4 border-white shadow-lg flex items-center justify-center">
              <span className="text-3xl font-bold text-primary-600">
                {(user.realName || user.username).charAt(0)}
              </span>
            </div>
            <div className="mb-2">
              <h1 className="text-2xl font-bold text-gray-900">{user.realName || user.username}</h1>
              <p className="text-gray-500">@{user.username}</p>
            </div>
            <span className="mb-3 px-3 py-1 bg-primary-100 text-primary-700 rounded-full text-sm font-medium">
              {roleMap[user.role]}
            </span>
          </div>

          <div className="grid grid-cols-3 gap-8">
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-primary-600">¥{user.totalDonation?.toLocaleString() || 0}</div>
              <div className="text-sm text-gray-500 mt-1">累计捐赠</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-green-600">{user.serviceHours || 0}</div>
              <div className="text-sm text-gray-500 mt-1">服务时长</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-blue-600">{donations.length}</div>
              <div className="text-sm text-gray-500 mt-1">捐赠次数</div>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-sm">
        <div className="border-b border-gray-100">
          <div className="flex gap-8 px-8">
            <button
              onClick={() => setActiveTab('info')}
              className={`py-4 font-medium ${activeTab === 'info' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              个人信息
            </button>
            <button
              onClick={() => setActiveTab('donations')}
              className={`py-4 font-medium ${activeTab === 'donations' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              捐赠记录
            </button>
            <button
              onClick={() => setActiveTab('refunds')}
              className={`py-4 font-medium ${activeTab === 'refunds' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              退款申请{refunds.length > 0 ? ` (${refunds.length})` : ''}
            </button>
          </div>
        </div>

        <div className="p-8">
          {activeTab === 'info' && (
            <div className="max-w-lg">
              <div className="space-y-6">
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">用户名</label>
                  <div className="text-gray-900">{user.username}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">邮箱</label>
                  <div className="text-gray-900">{user.email}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">真实姓名</label>
                  <div className="text-gray-900">{user.realName || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">手机号</label>
                  <div className="text-gray-900">{user.phone || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">注册时间</label>
                  <div className="text-gray-900">{user.createdAt ? new Date(user.createdAt).toLocaleDateString() : '-'}</div>
                </div>
              </div>
              <button
                onClick={logout}
                className="mt-8 px-6 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
              >
                退出登录
              </button>
            </div>
          )}

          {activeTab === 'donations' && (
            <div>
              {loading ? (
                <div className="text-center py-8">加载中...</div>
              ) : donations.length === 0 ? (
                <div className="text-center py-12 text-gray-500">暂无捐赠记录</div>
              ) : (
                <div className="space-y-4">
                  {donations.map((donation) => {
                    const refundStatus: RefundStatus | undefined = donation.refund?.status;
                    const isRefunded = donation.paymentStatus === 'refunded';
                    const frozen = refundStatus === 'pending';
                    return (
                      <div key={donation.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
                        <div className="flex-1">
                          <div className="flex items-center gap-2">
                            <span className="font-medium text-gray-900">{donation.project?.title}</span>
                            {isRefunded && (
                              <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-gray-200 text-gray-600">已退款</span>
                            )}
                            {renderRefundBadge(refundStatus)}
                          </div>
                          <div className="text-sm text-gray-500">
                            {new Date(donation.createdAt).toLocaleString()}
                          </div>
                          {donation.certificateNo && !frozen && !isRefunded && (
                            <div className="text-sm text-primary-600">凭证号：{donation.certificateNo}</div>
                          )}
                          {frozen && (
                            <div className="text-xs text-amber-600 mt-1">退款审核中，凭证已冻结，筹款进度暂不扣减</div>
                          )}
                          {isRefunded && (
                            <div className="text-xs text-gray-500 mt-1">该笔捐赠已退款，凭证已作废</div>
                          )}
                        </div>
                        <div className="text-right">
                          <div className={`text-xl font-bold ${isRefunded ? 'text-gray-400 line-through' : 'text-primary-600'}`}>
                            ¥{donation.amount.toLocaleString()}
                          </div>
                          {!isRefunded && !frozen && donation.certificateNo && (
                            <button
                              onClick={() => viewCertificate(donation.id)}
                              className="text-sm text-primary-600 hover:text-primary-700 block ml-auto"
                            >
                              查看凭证
                            </button>
                          )}
                          {frozen && <div className="text-xs text-amber-600 mt-1">凭证冻结中</div>}
                          {/* 24 小时内、未申请过退款的成功捐赠可申请全额退款 */}
                          {!isRefunded && !refundStatus && withinRefundWindow(donation) && (
                            <button
                              onClick={() => setRefundTarget(donation)}
                              className="text-sm text-red-500 hover:text-red-600 block ml-auto mt-1"
                            >
                              申请退款
                            </button>
                          )}
                          {refundStatus === 'rejected' && (
                            <button
                              onClick={() => setActiveTab('refunds')}
                              className="text-sm text-gray-500 hover:text-gray-700 block ml-auto mt-1"
                            >
                              查看驳回原因
                            </button>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}

          {activeTab === 'refunds' && (
            <div>
              {loading ? (
                <div className="text-center py-8">加载中...</div>
              ) : refunds.length === 0 ? (
                <div className="text-center py-12 text-gray-500">暂无退款申请</div>
              ) : (
                <div className="space-y-4">
                  {refunds.map((r) => (
                    <div key={r.id} className="p-5 bg-gray-50 rounded-xl">
                      <div className="flex items-center justify-between mb-2">
                        <div className="font-medium text-gray-900">{r.donation?.project?.title || `捐赠 #${r.donationId}`}</div>
                        {renderRefundBadge(r.status)}
                      </div>
                      <div className="text-sm text-gray-500 space-y-1">
                        <div>退款金额：<span className="text-primary-600 font-semibold">¥{r.donation?.amount?.toLocaleString()}</span>（全额）</div>
                        <div>申请时间：{new Date(r.createdAt).toLocaleString()}</div>
                        <div>申请原因：{r.reason}</div>
                        {r.status !== 'pending' && (
                          <>
                            <div>处理时间：{r.reviewedAt ? new Date(r.reviewedAt).toLocaleString() : '-'}</div>
                            {r.reviewNote && <div>处理备注：{r.reviewNote}</div>}
                          </>
                        )}
                        {r.status === 'pending' && <div className="text-amber-600">等待管理员审核，凭证冻结中</div>}
                        {r.status === 'approved' && <div className="text-green-600">退款已批准，筹款金额与个人累计已扣减，凭证已作废</div>}
                        {r.status === 'rejected' && <div className="text-red-600">退款申请已驳回，凭证恢复正常</div>}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {refundTarget && (
        <RefundApplyModal
          donation={refundTarget}
          isOpen={!!refundTarget}
          onClose={() => setRefundTarget(null)}
          onSuccess={loadData}
        />
      )}
    </div>
  );
};

export default Profile;
