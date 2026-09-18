export interface User {
  id: string;
  username: string;
  email: string;
  role: 'user' | 'org' | 'admin';
  realName?: string;
  avatar?: string;
  phone?: string;
  totalDonation: number;
  serviceHours: number;
  createdAt?: string;
}

export interface Organization {
  id: string;
  userId: string;
  name: string;
  description?: string;
  licenseNumber?: string;
  contactPerson?: string;
  contactPhone?: string;
  address?: string;
  status: 'pending' | 'approved' | 'rejected';
}

export interface Project {
  id: string;
  organizationId: string;
  title: string;
  description?: string;
  category: 'education' | 'elderly' | 'medical' | 'disaster' | 'environment' | 'other';
  targetAmount: number;
  currentAmount: number;
  executionPlan?: string;
  coverImage?: string;
  status: 'pending' | 'approved' | 'rejected' | 'completed';
  startDate?: string;
  endDate?: string;
  createdAt: string;
  progress: number;
  organization?: Organization;
}

export interface Donation {
  id: string;
  userId: string;
  projectId: string;
  amount: number;
  paymentMethod: 'wechat' | 'alipay' | 'bank';
  paymentStatus: 'pending' | 'success' | 'failed' | 'refunded';
  certificateNo?: string;
  isAnonymous: boolean;
  message?: string;
  createdAt: string;
  project?: Project;
  donorName?: string;
  refund?: RefundApplication;
}

export type RefundStatus = 'pending' | 'approved' | 'rejected';

export interface RefundApplication {
  id: string;
  donationId: string;
  userId: string;
  projectId: string;
  amount: number;
  reason: string;
  status: RefundStatus;
  reviewReason?: string;
  reviewerId?: number;
  reviewedAt?: string;
  createdAt: string;
  updatedAt?: string;
  project?: Project;
  donation?: Donation;
}

export interface ProjectUpdate {
  id: string;
  projectId: string;
  title: string;
  content?: string;
  images?: string;
  createdAt: string;
}

export interface RankingItem {
  rank: number;
  userId: string;
  username: string;
  realName?: string;
  avatar?: string;
  totalDonation?: number;
  serviceHours?: number;
}
