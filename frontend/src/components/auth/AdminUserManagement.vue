<template>
  <div class="admin-user-management">
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
                <button class="btn btn-small btn-primary" @click="approveRegistration(reg.id)">
                  {{ t('admin.approve') }}
                </button>
                <button class="btn btn-small btn-danger" @click="rejectRegistration(reg.id)">
                  {{ t('admin.reject') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pagination">
        <button
          class="btn"
          :disabled="pendingPage === 1"
          @click="changePendingPage(pendingPage - 1)"
        >
          ‹ {{ t('admin.prevPage') }}
        </button>
        <span class="page-info">
          {{ t('admin.total') }} <span class="current-page">{{ pendingPage }}</span> /
          {{ Math.ceil(totalPending / pendingPageSize) || 1 }} {{ t('admin.page') }} ({{
            t('admin.total')
          }}
          {{ totalPending }} {{ t('admin.pending') }})
        </span>
        <button
          class="btn"
          :disabled="pendingPage >= Math.ceil(totalPending / pendingPageSize)"
          @click="changePendingPage(pendingPage + 1)"
        >
          {{ t('admin.nextPage') }} ›
        </button>
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
      <div class="pagination">
        <button class="btn" :disabled="currentPage === 1" @click="changePage(currentPage - 1)">
          ‹ {{ t('admin.prevPage') }}
        </button>
        <span class="page-info">
          {{ t('admin.total') }} <span class="current-page">{{ currentPage }}</span> /
          {{ Math.ceil(totalUsers / pageSize) || 1 }} {{ t('admin.page') }} ({{ t('admin.total') }}
          {{ totalUsers }} {{ t('admin.users') }})
        </span>
        <button
          class="btn"
          :disabled="currentPage >= Math.ceil(totalUsers / pageSize)"
          @click="changePage(currentPage + 1)"
        >
          {{ t('admin.nextPage') }} ›
        </button>
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
          <label>{{ t('admin.maxAITokens') }}</label>
          <input v-model.number="quotaForm.max_ai_tokens" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxAIConcurrency') }}</label>
          <input v-model.number="quotaForm.max_ai_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxFeedFetchConcurrency') }}</label>
          <input v-model.number="quotaForm.max_feed_fetch_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxDBQueryConcurrency') }}</label>
          <input v-model.number="quotaForm.max_db_query_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxMediaCacheConcurrency') }}</label>
          <input v-model.number="quotaForm.max_media_cache_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxRSSDiscoveryConcurrency') }}</label>
          <input v-model.number="quotaForm.max_rss_discovery_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxRSSPathCheckConcurrency') }}</label>
          <input v-model.number="quotaForm.max_rss_path_check_concurrency" type="number" min="0" />
        </div>
        <div class="form-group">
          <label>{{ t('admin.maxTranslationConcurrency') }}</label>
          <input v-model.number="quotaForm.max_translation_concurrency" type="number" min="0" />
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
            {{ t('admin.maxAITokens') }}: {{ userQuota?.used_ai_tokens || 0 }} {{ t('admin.of') }}
            {{ userQuota?.max_ai_tokens || 0 }}
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

const currentPage = ref(1);
const pageSize = ref(5);
const totalUsers = ref(0);

const pendingPage = ref(1);
const pendingPageSize = ref(5);
const totalPending = ref(0);

const editForm = ref({
  username: '',
  email: '',
  role: 'user' as const,
  status: 'active' as const,
});

const quotaForm = ref({
  max_feeds: 100,
  max_articles: 100000,
  max_ai_tokens: 1000000,
  max_ai_concurrency: 5,
  max_feed_fetch_concurrency: 3,
  max_db_query_concurrency: 5,
  max_media_cache_concurrency: 5,
  max_rss_discovery_concurrency: 8,
  max_rss_path_check_concurrency: 5,
  max_translation_concurrency: 3,
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
    const [regs, usersData, tpl] = await Promise.all([
      authApi.admin
        .getPendingRegistrations(pendingPage.value, pendingPageSize.value)
        .catch(() => ({ registrations: [], total: 0 })),
      authApi.admin
        .getUsers(currentPage.value, pageSize.value)
        .catch(() => ({ users: [], total: 0 })),
      authApi.admin.getTemplateUser().catch(() => null),
    ]);
    console.log('Users loaded:', usersData);
    console.log('Pending registrations:', regs);
    pendingRegistrations.value = regs.registrations || [];
    totalPending.value = regs.total || 0;
    users.value = usersData.users || [];
    totalUsers.value = usersData.total || 0;
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
  const confirmed = await window.showConfirm({
    title: t('admin.confirm'),
    message: t('admin.confirmDelete'),
    isDanger: true,
  });
  if (!confirmed) return;
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
      max_ai_tokens: userQuota.value.max_ai_tokens,
      max_ai_concurrency: userQuota.value.max_ai_concurrency,
      max_feed_fetch_concurrency: userQuota.value.max_feed_fetch_concurrency,
      max_db_query_concurrency: userQuota.value.max_db_query_concurrency,
      max_media_cache_concurrency: userQuota.value.max_media_cache_concurrency ?? 5,
      max_rss_discovery_concurrency: userQuota.value.max_rss_discovery_concurrency ?? 8,
      max_rss_path_check_concurrency: userQuota.value.max_rss_path_check_concurrency ?? 5,
      max_translation_concurrency: userQuota.value.max_translation_concurrency ?? 3,
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

const changePage = (page: number) => {
  currentPage.value = page;
  loadData();
};

const changePendingPage = (page: number) => {
  pendingPage.value = page;
  loadData();
};
</script>

<style scoped>
.admin-user-management {
  padding: 5px 20px;
  max-width: 1200px;
  margin: 0 auto;
}

h2 {
  margin-bottom: 20px;
  color: #333;
}

h3 {
  margin: 10px 0 10px;
  color: #555;
  border-bottom: 2px solid #eee;
  padding-bottom: 5px;
}

.error-message {
  background: #fee;
  color: #c33;
  padding: 10px;
  border-radius: 4px;
  margin-bottom: 10px;
}

.section {
  margin-bottom: 15px;
}

.empty-state {
  color: #999;
  text-align: center;
  padding: 40px;
}

.table-container {
  overflow-x: auto;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.table-container::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.table-container::-webkit-scrollbar-track {
  background: #f1f1f1;
}

.table-container::-webkit-scrollbar-thumb {
  background: #888;
  border-radius: 4px;
}

.table-container::-webkit-scrollbar-thumb:hover {
  background: #555;
}

table {
  width: 100%;
  border-collapse: collapse;
  background: white;
}

table th {
  background: linear-gradient(180deg, #f8f9fa 0%, #e9ecef 100%);
  font-weight: 600;
  color: #495057;
  padding: 12px 10px;
  text-align: left;
  border-bottom: 2px solid #dee2e6;
  white-space: nowrap;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

table th:nth-child(1) {
  width: 50px;
  min-width: 50px;
}
table th:nth-child(2) {
  width: 100px;
  min-width: 100px;
}
table th:nth-child(3) {
  width: 160px;
  min-width: 160px;
}
table th:nth-child(4) {
  width: 80px;
  min-width: 80px;
}
table th:nth-child(5) {
  width: 80px;
  min-width: 80px;
}
table th:nth-child(6) {
  width: 70px;
  min-width: 70px;
  text-align: center;
}
table th:nth-child(7) {
  width: 145px;
  min-width: 145px;
}
table th:nth-child(8) {
  width: 220px;
  min-width: 220px;
  text-align: center;
  white-space: nowrap;
}

.section:first-child table th:nth-child(1) {
  width: 120px;
  min-width: 120px;
}
.section:first-child table th:nth-child(2) {
  width: 180px;
  min-width: 180px;
}
.section:first-child table th:nth-child(3) {
  width: 140px;
  min-width: 140px;
}
.section:first-child table th:nth-child(4) {
  width: 180px;
  min-width: 180px;
  text-align: center;
  white-space: nowrap;
}

.section:first-child td:nth-child(4) {
  text-align: center;
}

td {
  padding: 10px 8px;
  text-align: left;
  border-bottom: 1px solid #f0f0f0;
  font-size: 13px;
  color: #343a40;
}

td:nth-child(6) {
  text-align: center;
}
td:nth-child(8) {
  text-align: center;
}

tr:hover {
  background: #f8f9fa;
}

tr:last-child td {
  border-bottom: none;
}

.actions {
  display: flex;
  gap: 4px;
  justify-content: center;
  flex-wrap: nowrap;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  font-weight: 500;
}

.btn-primary {
  background: linear-gradient(180deg, #007bff 0%, #0056b3 100%);
  color: white;
  box-shadow: 0 2px 4px rgba(0, 123, 255, 0.2);
}

.btn-primary:hover {
  background: linear-gradient(180deg, #0056b3 0%, #004085 100%);
  box-shadow: 0 4px 8px rgba(0, 123, 255, 0.3);
  transform: translateY(-1px);
}

.btn-danger {
  background: linear-gradient(180deg, #dc3545 0%, #bd2130 100%);
  color: white;
  box-shadow: 0 2px 4px rgba(220, 53, 69, 0.2);
}

.btn-danger:hover {
  background: linear-gradient(180deg, #bd2130 0%, #921b24 100%);
  box-shadow: 0 4px 8px rgba(220, 53, 69, 0.3);
  transform: translateY(-1px);
}

.btn-small {
  padding: 4px 8px;
  font-size: 11px;
  border-radius: 3px;
}

.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  transform: none !important;
  box-shadow: none !important;
}

.role-badge,
.status-badge {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

.role-badge.admin {
  background: linear-gradient(180deg, #ffc107 0%, #e0a800 100%);
  color: #212529;
  box-shadow: 0 1px 2px rgba(255, 193, 7, 0.3);
}

.role-badge.user {
  background: linear-gradient(180deg, #17a2b8 0%, #117a8b 100%);
  color: white;
  box-shadow: 0 1px 2px rgba(23, 162, 184, 0.3);
}

.role-badge.template {
  background: linear-gradient(180deg, #6f42c1 0%, #5a32a3 100%);
  color: white;
  box-shadow: 0 1px 2px rgba(111, 66, 193, 0.3);
}

.status-badge.active {
  background: linear-gradient(180deg, #28a745 0%, #1e7e34 100%);
  color: white;
  box-shadow: 0 1px 2px rgba(40, 167, 69, 0.3);
}

.status-badge.pending {
  background: linear-gradient(180deg, #ffc107 0%, #e0a800 100%);
  color: #212529;
  box-shadow: 0 1px 2px rgba(255, 193, 7, 0.3);
}

.status-badge.suspended {
  background: linear-gradient(180deg, #dc3545 0%, #bd2130 100%);
  color: white;
  box-shadow: 0 1px 2px rgba(220, 53, 69, 0.3);
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

.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  margin-top: 15px;
  padding: 12px;
  background: #f8f9fa;
  border-radius: 8px;
}

.pagination .btn {
  padding: 6px 14px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.pagination .btn:hover:not(:disabled) {
  background: #0056b3;
}

.pagination .btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.page-info {
  color: #555;
  font-size: 13px;
  font-weight: 500;
}

.page-info .current-page {
  color: #007bff;
  font-weight: 600;
}
</style>
