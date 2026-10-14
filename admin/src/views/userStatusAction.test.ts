import { describe, expect, it, vi } from 'vitest'

import { UserStatus, type AdminUserRow } from '@/api/admin'
import { applyUserStatusAction, userStatusActionText, userStatusText, type UserStatusApi } from '@/views/userStatusAction'

function user(status: UserStatus): AdminUserRow {
  return {
    id: 7,
    account: 'member-under-test',
    nickname: 'Member',
    avatar: '',
    role: 2,
    status,
    isOnline: false,
    sendMessageCount: 0,
    groupAdminCount: 0,
    groupCount: 0,
    curtailChat: false,
    curtailAddUser: false,
    curtailCreateGroup: false,
    curtailInGroupChat: false,
    createdAt: '2026-01-01T00:00:00Z',
  }
}

function api(): UserStatusApi {
  return {
    disableUser: vi.fn().mockResolvedValue({ disabled: true, status: UserStatus.Disabled }),
    enableUser: vi.fn().mockResolvedValue({ enabled: true, status: UserStatus.Active }),
  }
}

describe('Admin user status action', () => {
  it('shows 禁用 for an enabled user', () => {
    expect(userStatusText(UserStatus.Active)).toBe('启用')
    expect(userStatusActionText(UserStatus.Active)).toBe('禁用')
  })

  it('shows 启用 for a disabled user', () => {
    expect(userStatusText(UserStatus.Disabled)).toBe('禁用')
    expect(userStatusActionText(UserStatus.Disabled)).toBe('启用')
  })

  it('synchronizes the row and action after Disable succeeds', async () => {
    const row = user(UserStatus.Active)
    const client = api()
    await applyUserStatusAction(row, client)
    expect(client.disableUser).toHaveBeenCalledWith(row.id)
    expect(row.status).toBe(UserStatus.Disabled)
    expect(userStatusActionText(row.status)).toBe('启用')
  })

  it('restores the row and action after Enable succeeds', async () => {
    const row = user(UserStatus.Disabled)
    const client = api()
    await applyUserStatusAction(row, client)
    expect(client.enableUser).toHaveBeenCalledWith(row.id)
    expect(row.status).toBe(UserStatus.Active)
    expect(userStatusActionText(row.status)).toBe('禁用')
  })

  it('does not change the row when the API fails', async () => {
    const row = user(UserStatus.Active)
    const client = api()
    vi.mocked(client.disableUser).mockRejectedValue(new Error('internal detail'))
    await expect(applyUserStatusAction(row, client)).rejects.toThrow()
    expect(row.status).toBe(UserStatus.Active)
    expect(userStatusActionText(row.status)).toBe('禁用')
  })
})
