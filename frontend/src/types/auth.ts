export type Role = 'admin' | 'annotator' | 'data_manager' | 'adjudicator' | 'auditor';

export interface UserView {
  id: number;
  username: string;
  role: Role;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: UserView;
}
