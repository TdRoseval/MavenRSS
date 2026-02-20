<template>
  <div class="admin-user-management">
    <h2>{{ t('admin.title') }}</h2>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div class="section">
      <h3>{{ t('admin.pendingRegistrations') }}</h3>
      <div v-if="pendingRegistrations.length === 0" class="empty-state">
        {{ t('admin.noPendingRegistrations') }}
      </div>
      <div v-else class="table-container">
        <table>
          <thead>
            <tr>
              <th>{{ t('admin.username') }}</th>
              <th>{{ t('admin.email') }}</th>
              <th>{{ t('admin.createdAt') }}</th>
              <th>{{ t('admin.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="reg in pendingRegistrations" :key="reg.id">
              <td>{{ reg.username }}</td>
              <td>{{ reg.email }}</td>
              <td>{{ formatDate(reg.created_at) }}</td>
              <td class="actions">
                <button class="btn btn-primary" @click="approveRegistration(reg.id)">
                  {{ t('admin.approve') }}
                </button>
                <button class="btn btn-danger" @click="rejectRegistration(reg.id)">
                  {{ t('admin.reject') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="section">
      <h3>{{ t('admin.userList') }}</h3>
      <div v-if="users.length === 0" class="empty-state">
        {{ t('admin.noUsers') }}
      </div>
      <div v-else class="table-container">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>{{ t('admin.username') }}</th>
              <th>{{ t('admin.email') }}</th>
              <th>{{ t('admin.role') }}</th>
              <th>{{ t('admin.status') }}</th>
              <th>{{ t('admin.inherited') }}</th>
              <th>{{ t('admin.createdAt') }}</th>
              <th>{{ t('admin.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td>{{ user.id }}</td>
              <td>{{ user.username }}</td>
              <td>{{ user.email }}</td>
              <td>
                <span :class="['role-badge', user.role]">
                  {{ getRoleLabel(user.role) }}
                </span>
              </td>
              <td>
                <span :class="['status-badge', user.status]">
                  {{ getStatusLabel(user.status) }}
                </span>
              </td>
              <td>{{ user.has_inherited ? t('admin.yes') : t('admin.no') }}</td>
              <td>{{ formatDate(user.created_at) }}</td>
              <td class="actions">
                <button
                  class="btn btn-small"
                  :disabled="user.role === 'template'"
                  @click="editUser(user)"
                >
                  {{ t('admin.edit') }}
                </button>
                <button class="btn btn-small" @click="viewQuota(user)">
                  {{ t('admin.quota') }}
                </button>
                <button
                  class="btn btn-small btn-danger"
                  :disabled="user.role === 'admin' || user.role === 'template'"
                  @click="deleteUser(user.id)"
                >
                  {{ t('admin.delete') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="selectedUser" class="section">
      <h3>{{ t('admin.editUser') }}</h3>
      <div class="form-container">
        <div class="form-group">
          <label>{{ t('admin.username') }}</label>
          <input v-model="editForm.username" type="text" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.email') }}</label>
          <input v-model="editForm.email" type="email" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.role') }}</label>
          <select v-model="editForm.role">
            <option value="user">{{ t('admin.roles.user') }}</option>
            <option value="admin">{{ t('admin.roles.admin') }}</option>
          </select>
        </div>
        <div class="form-group">
          <label>{{ t('admin.status') }}</label>
          <select v-model="editForm.status">
            <option value="active">{{ t('admin.statuses.active') }}</option>
            <option value="pending">{{ t('admin.statuses.pending') }}</option>
            <option value="suspended">{{ t('admin.statuses.suspended') }}</option>
          </select>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" @click="saveUser">{{ t('admin.save') }}</button>
          <button class="btn" @click="selectedUser = null">{{ t('admin.cancel') }}</button>
        </div>
      </div>
    </div>

    <div v-if="selectedQuotaUser" class="section">
      <h3>{{ t('admin.quota') }} - {{ selectedQuotaUser.username }}</h3>
      <div class="form-container">
        <div class="form-group">
          <label>{{ t('admin.maxFeeds') }}</label>
          <input v-model.number="quotaForm.max_feeds" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxArticles') }}</label>
          <input v-model.number="quotaForm.max_articles" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxAICallsPerDay') }}</label>
          <input v-model.number="quotaForm.max_ai_calls_per_day" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxStorageMB') }}</label>
          <input v-model.number="quotaForm.max_storage_mb" type="number" min="0" />
        </div>
        <div class="quota-usage">
          <h4>{{ t('admin.used') }}</h4>
          <p>
            {{ t('admin.maxFeeds') }}: {{ userQuota?.used_feeds || 0 }} {{ t('admin.of') }}
            {{ userQuota?.max_feeds || 0 }}
          </p>
          <p>
            {{ t('admin.maxArticles') }}: {{ userQuota?.used_articles || 0 }} {{ t('admin.of') }}
            {{ userQuota?.max_articles || 0 }}
          </p>
          <p>
            {{ t('admin.maxAICallsPerDay') }}: {{ userQuota?.used_ai_calls_today || 0 }}
            {{ t('admin.of') }} {{ userQuota?.max_ai_calls_per_day || 0 }}
          </p>
          <p>
            {{ t('admin.maxStorageMB') }}: {{ userQuota?.used_storage_mb || 0 }} MB
            {{ t('admin.of') }} {{ userQuota?.max_storage_mb || 0 }} MB
          </p>
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" @click="saveQuota">{{ t('admin.save') }}</button>
          <button class="btn" @click="selectedQuotaUser = null">{{ t('admin.cancel') }}</button>
        </div>
      </div>
    </div>

    <div class="section">
      <h3>{{ t('admin.templateUser') }}</h3>
      <div v-if="templateUser">
        <div class="template-info">
          <p>
            <strong>{{ t('admin.username') }}:</strong> {{ templateUser.username }}
          </p>
          <p>
            <strong>{{ t('admin.email') }}:</strong> {{ templateUser.email }}
          </p>
          <p>
            <strong>{{ t('admin.createdAt') }}:</strong> {{ formatDate(templateUser.created_at) }}
          </p>
        </div>
      </div>
      <div v-else class="empty-state">
        <p>{{ t('admin.noTemplateUser') }}</p>
        <button class="btn btn-primary" @click="showCreateTemplate = true">
          {{ t('admin.createTemplateUser') }}
        </button>
      </div>
    </div>

    <div v-if="showCreateTemplate" class="modal-overlay" @click.self="showCreateTemplate = false">
      <div class="modal">
        <h3>{{ t('admin.createTemplateUser') }}</h3>
        <div class="form-group">
          <label>{{ t('admin.username') }}</label>
          <input v-model="templateForm.username" type="text" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.email') }}</label>
          <input v-model="templateForm.email" type="email" />
        </div>
        <div class="form-group">
          <label>Password</label>
          <input v-model="templateForm.password" type="password" />
        </div>
        <div class="form-actions">
          <button class="btn btn-primary" @click="createTemplateUser">
            {{ t('admin.createTemplate') }}
          </button>
          <button class="btn" @click="showCreateTemplate = false">{{ t('admin.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '@/utils/authApi';
import type { User, UserQuota, PendingRegistration } from '@/types/auth';

const { t } = useI18n();

const error = ref('');
const pendingRegistrations = ref<PendingRegistration[]>([]);
const users = ref<User[]>([]);
const templateUser = ref<User | null>(null);
const selectedUser = ref<User | null>(null);
const selectedQuotaUser = ref<User | null>(null);
const userQuota = ref<UserQuota | null>(null);
const showCreateTemplate = ref(false);

const editForm = ref({
  username: '',
  email: '',
  role: 'user' as const,
  status: 'active' as const,
});

const quotaForm = ref({
  max_feeds: 100,
  max_articles: 100000,
  max_ai_calls_per_day: 100,
  max_storage_mb: 500,
});

const templateForm = ref({
  username: '',
  email: '',
  password: '',
});

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('zh-CN');
};

const getRoleLabel = (role: string) => {
  return t(`admin.roles.${role}`) || role;
};

const getStatusLabel = (status: string) => {
  return t(`admin.statuses.${status}`) || status;
};

const loadData = async () => {
  try {
    error.value = '';
    console.log('Loading admin data...');
    const [regs, usrs, tpl] = await Promise.all([
      authApi.admin.getPendingRegistrations().catch(() => []),
      authApi.admin.getUsers().catch(() => []),
      authApi.admin.getTemplateUser().catch(() => null),
    ]);
    console.log('Users loaded:', usrs);
    console.log('Pending registrations:', regs);
    pendingRegistrations.value = regs || [];
    users.value = usrs || [];
    templateUser.value = tpl;
  } catch (err) {
    console.error('Failed to load admin data:', err);
    error.value = err instanceof Error ? err.message : 'Failed to load data';
  }
};

const approveRegistration = async (id: number) => {
  try {
    await authApi.admin.approveRegistration(id);
    await loadData();
  } catch (err) {
    error.value =
      err instanceof Error ? err.message : t('admin.approveFailed') || 'Failed to approve';
  }
};

const rejectRegistration = async (id: number) => {
  try {
    await authApi.admin.rejectRegistration(id);
    await loadData();
  } catch (err) {
    error.value =
      err instanceof Error ? err.message : t('admin.rejectFailed') || 'Failed to reject';
  }
};

const editUser = (user: User) => {
  selectedUser.value = user;
  editForm.value = {
    username: user.username,
    email: user.email,
    role: user.role as 'user' | 'admin',
    status: user.status as 'active' | 'pending' | 'suspended',
  };
};

const saveUser = async () => {
  if (!selectedUser.value) return;
  try {
    await authApi.admin.updateUser(selectedUser.value.id, editForm.value);
    selectedUser.value = null;
    await loadData();
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('admin.saveFailed') || 'Failed to save';
  }
};

const deleteUser = async (id: number) => {
  if (!confirm(t('admin.confirmDelete'))) return;
  try {
    await authApi.admin.deleteUser(id);
    await loadData();
  } catch (err) {
    error.value =
      err instanceof Error ? err.message : t('admin.deleteFailed') || 'Failed to delete';
  }
};

const viewQuota = async (user: User) => {
  selectedQuotaUser.value = user;
  try {
    userQuota.value = await authApi.admin.getUserQuota(user.id);
    quotaForm.value = {
      max_feeds: userQuota.value.max_feeds,
      max_articles: userQuota.value.max_articles,
      max_ai_calls_per_day: userQuota.value.max_ai_calls_per_day,
      max_storage_mb: userQuota.value.max_storage_mb,
    };
  } catch (err) {
    error.value =
      err instanceof Error ? err.message : t('admin.getQuotaFailed') || 'Failed to get quota';
  }
};

const saveQuota = async () => {
  if (!selectedQuotaUser.value) return;
  try {
    userQuota.value = await authApi.admin.updateUserQuota(
      selectedQuotaUser.value.id,
      quotaForm.value
    );
    selectedQuotaUser.value = null;
  } catch (err) {
    error.value =
      err instanceof Error ? err.message : t('admin.saveQuotaFailed') || 'Failed to save quota';
  }
};

const createTemplateUser = async () => {
  try {
    await authApi.admin.createTemplateUser(templateForm.value);
    showCreateTemplate.value = false;
    templateForm.value = { username: '', email: '', password: '' };
    await loadData();
  } catch (err) {
    error.value =
      err instanceof Error
        ? err.message
        : t('admin.createTemplateFailed') || 'Failed to create template user';
  }
};

onMounted(() => {
  loadData();
});
</script>

<style scoped>
.admin-user-management {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}

h2 {
  margin-bottom: 20px;
  color: #333;
}

h3 {
  margin: 30px 0 15px;
  color: #555;
  border-bottom: 2px solid #eee;
  padding-bottom: 10px;
}

.error-message {
  background: #fee;
  color: #c33;
  padding: 10px;
  border-radius: 4px;
  margin-bottom: 20px;
}

.section {
  margin-bottom: 30px;
}

.empty-state {
  color: #999;
  text-align: center;
  padding: 40px;
}

.table-container {
  overflow-x: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
  background: white;
}

th,
td {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid #eee;
}

th {
  background: #f5f5f5;
  font-weight: 600;
}

tr:hover {
  background: #f9f9f9;
}

.actions {
  display: flex;
  gap: 8px;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.btn-primary {
  background: #007bff;
  color: white;
}

.btn-primary:hover {
  background: #0056b3;
}

.btn-danger {
  background: #dc3545;
  color: white;
}

.btn-danger:hover {
  background: #a71d2a;
}

.btn-small {
  padding: 4px 8px;
  font-size: 12px;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.role-badge,
.status-badge {
  display: inline-block;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.role-badge.admin {
  background: #ffc107;
  color: #333;
}

.role-badge.user {
  background: #17a2b8;
  color: white;
}

.role-badge.template {
  background: #6f42c1;
  color: white;
}

.status-badge.active {
  background: #28a745;
  color: white;
}

.status-badge.pending {
  background: #ffc107;
  color: #333;
}

.status-badge.suspended {
  background: #dc3545;
  color: white;
}

.form-container {
  background: white;
  padding: 20px;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.form-group {
  margin-bottom: 15px;
}

.form-group label {
  display: block;
  margin-bottom: 5px;
  font-weight: 500;
  color: #333;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
}

.form-actions {
  display: flex;
  gap: 10px;
  margin-top: 20px;
}

.quota-usage {
  background: #f5f5f5;
  padding: 15px;
  border-radius: 4px;
  margin: 15px 0;
}

.quota-usage h4 {
  margin: 0 0 10px;
  color: #555;
}

.quota-usage p {
  margin: 5px 0;
  color: #666;
}

.template-info {
  background: white;
  padding: 20px;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.template-info p {
  margin: 10px 0;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  padding: 30px;
  border-radius: 8px;
  min-width: 400px;
  max-width: 90%;
}

.modal h3 {
  margin-top: 0;
  border: none;
  padding: 0;
}
</style>
