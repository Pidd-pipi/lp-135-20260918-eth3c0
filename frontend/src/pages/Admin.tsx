import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { adminAPI, refundAPI } from '../api';
import { Project, Organization, RefundApplication } from '../types';

const Admin = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<'projects' | 'organizations' | 'refunds'>('projects');
  const [pendingProjects, setPendingProjects] = useState<Project[]>([]);
  const [pendingOrgs, setPendingOrgs] = useState<Organization[]>([]);
  const [refunds, setRefunds] = useState<RefundApplication[]>([]);
  const [refundFilter, setRefundFilter] = useState<'pending' | 'all'>('pending');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user?.role === 'admin') {
      loadPendingItems();
    }
  }, [user]);

  useEffect(() => {
    if (user?.role === 'admin' && activeTab === 'refunds') {
      loadRefunds();
    }
  }, [activeTab, refundFilter, user]);

  const loadPendingItems = async () => {
    try {
      const [projectsRes, orgsRes] = await Promise.all([
        adminAPI.getPendingProjects(),
        adminAPI.getPendingOrganizations(),
      ]);
      setPendingProjects(projectsRes.data.projects);
      setPendingOrgs(orgsRes.data.organizations);
    } catch (error) {
      console.error('加载待审核项目失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadRefunds = async () => {
    try {
      const res = await refundAPI.adminList({ status: refundFilter === 'pending' ? 'pending' : '', limit: 50 });
      setRefunds(res.data.refunds || []);
    } catch (error) {
      console.error('加载退款申请失败:', error);
    }
  };

  const handleReviewProject = async (projectId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewProject(projectId, { status, comment: '' });
      alert(`项目已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error) {
      alert('操作失败');
    }
  };

  const handleReviewOrg = async (orgId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewOrganization(orgId, { status, comment: '' });
      alert(`组织已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error) {
      alert('操作失败');
    }
  };

  const handleReviewRefund = async (refund: RefundApplication, status: 'approved' | 'rejected') => {
    const confirmMsg = status === 'approved'
      ? `确认批准该笔 ¥${refund.donation?.amount} 的全额退款吗？\n批准后将扣减项目已筹与个人累计、作废凭证；若项目跌破目标将恢复募集中。`
      : '确认驳回该退款申请吗？驳回后凭证将恢复正常展示。';
    if (!window.confirm(confirmMsg)) return;

    let note = '';
    if (status === 'rejected') {
      note = window.prompt('请输入驳回原因（选填）：') || '';
    }
    try {
      const res = await refundAPI.review(refund.id, { status, note });
      alert(res.data?.message || '处理完成');
      loadRefunds();
    } catch (error: any) {
      alert(error.response?.data?.message || '操作失败');
    }
  };

  if (!user || user.role !== 'admin') {
    return <Navigate to="/" />;
  }

  const categoryMap: Record<string, string> = {
    education: '助学',
    elderly: '助老',
    medical: '医疗',
    disaster: '救灾',
    environment: '环保',
    other: '其他',
  };

  const refundStatusMeta: Record<string, { label: string; cls: string }> = {
    pending: { label: '待审核', cls: 'bg-amber-100 text-amber-700' },
    approved: { label: '已批准', cls: 'bg-green-100 text-green-700' },
    rejected: { label: '已驳回', cls: 'bg-red-100 text-red-700' },
  };

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">管理后台</h1>

      <div className="flex gap-4 mb-8">
        <button
          onClick={() => setActiveTab('projects')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'projects'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          待审核项目 ({pendingProjects.length})
        </button>
        <button
          onClick={() => setActiveTab('organizations')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'organizations'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          待审核组织 ({pendingOrgs.length})
        </button>
        <button
          onClick={() => setActiveTab('refunds')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'refunds'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          退款审核
        </button>
      </div>

      {loading ? (
        <div className="text-center py-20">加载中...</div>
      ) : activeTab === 'projects' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingProjects.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核项目</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingProjects.map((project) => (
                <div key={project.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded text-xs font-medium">
                          {categoryMap[project.category]}
                        </span>
                        <span className="text-sm text-gray-500">
                          发布时间：{new Date(project.createdAt).toLocaleDateString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{project.title}</h3>
                      <p className="text-gray-600 mb-2">{project.description}</p>
                      <div className="text-sm text-gray-500">
                        目标金额：¥{project.targetAmount.toLocaleString()}
                      </div>
                      <div className="text-sm text-gray-500">
                        执行计划：{project.executionPlan}
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewProject(project.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewProject(project.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : activeTab === 'organizations' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingOrgs.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核组织</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingOrgs.map((org) => (
                <div key={org.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{org.name}</h3>
                      <p className="text-gray-600 mb-2">{org.description}</p>
                      <div className="grid grid-cols-2 gap-4 text-sm text-gray-500">
                        <div>执照编号：{org.licenseNumber || '-'}</div>
                        <div>联系人：{org.contactPerson || '-'}</div>
                        <div>联系电话：{org.contactPhone || '-'}</div>
                        <div>地址：{org.address || '-'}</div>
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewOrg(org.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewOrg(org.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div>
          <div className="flex gap-2 mb-4">
            <button
              onClick={() => setRefundFilter('pending')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium ${
                refundFilter === 'pending' ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
              }`}
            >
              待审核
            </button>
            <button
              onClick={() => setRefundFilter('all')}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium ${
                refundFilter === 'all' ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
              }`}
            >
              全部
            </button>
          </div>
          <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
            {refunds.length === 0 ? (
              <div className="text-center py-12 text-gray-500">暂无退款申请</div>
            ) : (
              <div className="divide-y divide-gray-100">
                {refunds.map((r) => {
                  const meta = refundStatusMeta[r.status];
                  return (
                    <div key={r.id} className="p-6">
                      <div className="flex justify-between items-start">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <span className={`px-2 py-1 rounded text-xs font-medium ${meta.cls}`}>{meta.label}</span>
                            <span className="text-sm text-gray-500">
                              申请时间：{new Date(r.createdAt).toLocaleString()}
                            </span>
                            <span className="text-sm text-gray-500">
                              捐赠时间：{r.donation ? new Date(r.donation.createdAt).toLocaleString() : '-'}
                            </span>
                          </div>
                          <h3 className="text-lg font-semibold text-gray-900 mb-1">
                            {r.donation?.project?.title || `捐赠 #${r.donationId}`}
                          </h3>
                          <div className="grid grid-cols-2 gap-x-8 gap-y-1 text-sm text-gray-500 mt-2">
                            <div>捐赠人：{r.user?.realName || r.user?.username || `用户#${r.userId}`}（@{r.user?.username || '-'}）</div>
                            <div>退款金额：<span className="text-primary-600 font-semibold">¥{r.donation?.amount?.toLocaleString()}</span>（全额）</div>
                            <div className="col-span-2">退款原因：{r.reason}</div>
                            {r.status !== 'pending' && (
                              <>
                                <div>处理人：{r.reviewer?.realName || r.reviewer?.username || `管理员#${r.reviewerId}`}</div>
                                <div>处理时间：{r.reviewedAt ? new Date(r.reviewedAt).toLocaleString() : '-'}</div>
                                {r.reviewNote && <div className="col-span-2">处理备注：{r.reviewNote}</div>}
                              </>
                            )}
                          </div>
                        </div>
                        {r.status === 'pending' && (
                          <div className="flex gap-2 ml-6">
                            <button
                              onClick={() => handleReviewRefund(r, 'approved')}
                              className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                            >
                              批准
                            </button>
                            <button
                              onClick={() => handleReviewRefund(r, 'rejected')}
                              className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                            >
                              驳回
                            </button>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default Admin;
