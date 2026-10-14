import { UserStatus, type AdminUserRow } from '@/api/admin'

export interface UserStatusApi {
  disableUser(userId: number): Promise<{ disabled: boolean; status: UserStatus }>
  enableUser(userId: number): Promise<{ enabled: boolean; status: UserStatus }>
}

export function userStatusText(status: UserStatus): string {
  return status === UserStatus.Active ? '启用' : '禁用'
}

export function userStatusActionText(status: UserStatus): string {
  return status === UserStatus.Active ? '禁用' : '启用'
}

export async function applyUserStatusAction(row: AdminUserRow, api: UserStatusApi): Promise<UserStatus> {
  const response = row.status === UserStatus.Active
    ? await api.disableUser(row.id)
    : await api.enableUser(row.id)
  row.status = response.status
  return response.status
}
