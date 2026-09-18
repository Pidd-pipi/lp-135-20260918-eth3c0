import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { donationAPI } from '../api';
import { Donation, RefundApplication } from '../types';
import RefundModal from '../components/RefundModal';
import { refundStatusMeta, canApplyRefund } from '../util/refund';

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
        donationAPI.getMyDonations({ limit: 50 }),
        donationAPI.getMyRefunds({ limit: 50 }),
      ]);
      setDonations(donRes.data.donations);
      setRefunds(refundRes.data.refunds);
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
      // 退款审核中冻结凭证；已退款凭证作废；驳回恢复展示
      const status = error.response?.status;
      const msg = error.response?.data?.message || '获取凭证失败';
      alert(status === 409 ? msg : '获取凭证失败');
    }
  };

  const handleApplyRefund = (donation: Donation) => {
    if (donation.refund?.status === 'pending') {
      alert('该笔捐赠的退款申请正在审核中');
      return;
    }
    if (donation.refund) {
      alert('该笔捐赠的退款申请已处理，不能再次申请');
      return;
    }
    if (!canApplyRefund(donation.createdAt)) {
      alert('已超过捐赠成功后 24 小时退款申请时限');
      return;
    }
    setRefundTarget(donation);
  };

  if (!user) {
    return <Navigate to="/login" />;
  }

  const roleMap: Record<string, string> = {
    user: '个人用户',
    org: '公益组织',
    admin: '管理员',
  };

  const pendingCount = refunds.filter((r) => r.status === 'pending').length;

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
            {user.role === 'user' && (
              <button
                onClick={() => setActiveTab('refunds')}
                className={`py-4 font-medium ${activeTab === 'refunds' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
              >
                退款申请{pendingCount > 0 && <span className="ml-1 text-amber-600">({pendingCount} 审核中)</span>}
              </button>
            )}
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
                <div className="text-center py-12 text-gray-500">
                  暂无捐赠记录
                </div>
              ) : (
                <div className="space-y-4">
                  {donations.map((donation) => {
                    const refund = donation.refund;
                    const withinWindow = canApplyRefund(donation.createdAt);
                    const certFrozen = refund?.status === 'pending';
                    return (
                      <div key={donation.id} className="p-4 bg-gray-50 rounded-xl">
                        <div className="flex items-center justify-between">
                          <div className="flex-1">
                            <div className="flex items-center gap-2">
                              <span className="font-medium text-gray-900">{donation.project?.title}</span>
                              {refund && (
                                <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${refundStatusMeta[refund.status].className}`}>
                                  退款{refundStatusMeta[refund.status].label}
                                </span>
                              )}
                            </div>
                            <div className="text-sm text-gray-500">
                              {new Date(donation.createdAt).toLocaleString()}
                            </div>
                            {donation.certificateNo && !certFrozen && (
                              <div className="text-sm text-primary-600">
                                凭证号：{donation.certificateNo}
                              </div>
                            )}
                            {certFrozen && (
                              <div className="text-sm text-amber-600">退款审核中，凭证暂时冻结</div>
                            )}
                          </div>
                          <div className="text-right">
                            <div className="text-xl font-bold text-primary-600">¥{donation.amount.toLocaleString()}</div>
                            <button
                              onClick={() => viewCertificate(donation.id)}
                              className="text-sm text-primary-600 hover:text-primary-700"
                            >
                              {certFrozen ? '凭证已冻结' : '查看凭证'}
                            </button>
                          </div>
                        </div>
                        {user.role === 'user' && (
                          <div className="mt-3 pt-3 border-t border-gray-200">
                            {refund?.status === 'pending' && (
                              <span className="text-sm text-amber-600">退款申请审核中，请耐心等待</span>
                            )}
                            {refund?.status === 'approved' && (
                              <span className="text-sm text-green-600">退款已批准，全额退款 ¥{refund.amount.toLocaleString()}，凭证已作废</span>
                            )}
                            {refund?.status === 'rejected' && (
                              <span className="text-sm text-red-600">退款申请已驳回{refund.reviewReason ? `：${refund.reviewReason}` : ''}，凭证已恢复</span>
                            )}
                            {!refund && (
                              withinWindow ? (
                                <button
                                  onClick={() => handleApplyRefund(donation)}
                                  className="text-sm px-3 py-1 bg-amber-50 text-amber-700 rounded-lg hover:bg-amber-100"
                                >
                                  申请退款（24 小时内）
                                </button>
                              ) : (
                                <span className="text-sm text-gray-400">已超过 24 小时退款时限</span>
                              )
                            )}
                          </div>
                        )}
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
                    <div key={r.id} className="p-4 bg-gray-50 rounded-xl">
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-1">
                            <span className="font-medium text-gray-900">{r.project?.title || `捐赠 #${r.donationId}`}</span>
                            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${refundStatusMeta[r.status].className}`}>
                              {refundStatusMeta[r.status].label}
                            </span>
                          </div>
                          <div className="text-sm text-gray-500">申请时间：{new Date(r.createdAt).toLocaleString()}</div>
                          <div className="text-sm text-gray-600 mt-1">退款金额：¥{r.amount.toLocaleString()}</div>
                          <div className="text-sm text-gray-600">申请原因：{r.reason}</div>
                          {r.reviewedAt && (
                            <div className="text-sm text-gray-500 mt-1">
                              处理时间：{new Date(r.reviewedAt).toLocaleString()}
                              {r.reviewReason ? `　处理意见：${r.reviewReason}` : ''}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <RefundModal
        donation={refundTarget as Donation}
        isOpen={!!refundTarget}
        onClose={() => setRefundTarget(null)}
        onSuccess={loadData}
      />
    </div>
  );
};

export default Profile;
