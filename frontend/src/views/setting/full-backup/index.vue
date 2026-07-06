<template>
    <div>
        <LayoutContent v-loading="loading" :title="$t('setting.fullBackup', 2)">
            <template #leftToolBar>
                <el-button type="primary" @click="onCreate">
                    {{ $t('commons.button.create') }}
                </el-button>
                <el-button type="primary" plain :disabled="selects.length === 0" @click="onBatchDelete">
                    {{ $t('commons.button.delete') }}
                </el-button>
            </template>
            <template #rightToolBar>
                <TableRefresh @search="search()" />
                <TableSetting title="fullbackup-refresh" @search="search()" />
            </template>
            <template #main>
                <ComplexTable
                    :pagination-config="paginationConfig"
                    v-model:selects="selects"
                    :data="data"
                    @search="search"
                >
                    <el-table-column type="selection" fix />
                    <el-table-column show-overflow-tooltip :label="$t('commons.table.name')" min-width="200" prop="name" fix />
                    <el-table-column :label="$t('commons.table.status')" prop="status" min-width="100">
                        <template #default="{ row }">
                            <el-tag :type="row.status === 'Success' ? 'success' : 'danger'">
                                {{ row.status }}
                            </el-tag>
                        </template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.size')" prop="size" min-width="100">
                        <template #default="{ row }">
                            {{ row.size ? formatFileSize(row.size) : '-' }}
                        </template>
                    </el-table-column>
                    <el-table-column :label="$t('commons.table.createdAt')" prop="createdAt" min-width="160" />
                    <el-table-column :label="$t('commons.table.description')" prop="description" min-width="160" />
                    <el-table-column :label="$t('commons.table.operate')" fix min-width="200">
                        <template #default="{ row }">
                            <el-button type="primary" link @click="onRestore(row)">
                                {{ $t('commons.button.recover') }}
                            </el-button>
                            <el-button type="primary" link @click="onEditDesc(row)">
                                {{ $t('commons.button.edit') }}
                            </el-button>
                            <el-button type="primary" link @click="onDelete(row)">
                                {{ $t('commons.button.delete') }}
                            </el-button>
                        </template>
                    </el-table-column>
                </ComplexTable>
            </template>
        </LayoutContent>

        <DialogPro v-model="createVisible" :title="$t('commons.button.create')" @confirm="submitCreate">
            <el-form ref="createFormRef" label-position="top" :model="createForm">
                <el-form-item :label="$t('commons.table.description')" prop="description">
                    <el-input v-model="createForm.description" type="textarea" />
                </el-form-item>
            </el-form>
        </DialogPro>

        <DialogPro v-model="editDescVisible" :title="$t('commons.button.edit')" @confirm="submitEditDesc">
            <el-form ref="editDescFormRef" label-position="top" :model="editDescForm">
                <el-form-item :label="$t('commons.table.description')" prop="description">
                    <el-input v-model="editDescForm.description" type="textarea" />
                </el-form-item>
            </el-form>
        </DialogPro>

        <DialogPro v-model="restoreVisible" :title="$t('commons.button.recover')" @confirm="submitRestore">
            <p>{{ $t('setting.restoreFullBackupConfirm') }}</p>
        </DialogPro>
    </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue';
import { createFullBackup, searchFullBackup, deleteFullBackup, updateFullBackupDescription, restoreFullBackup } from '@/api/modules/backup';
import { ElMessage } from 'element-plus';
import { formatFileSize } from '@/utils/misc';
import i18n from '@/lang';

const loading = ref(false);
const data = ref<any[]>([]);
const selects = ref<any[]>([]);
const paginationConfig = reactive({
    currentPage: 1,
    pageSize: 10,
    total: 0,
});

const createVisible = ref(false);
const createForm = reactive({ description: '' });
const createFormRef = ref();

const editDescVisible = ref(false);
const editDescForm = reactive({ id: 0, description: '' });
const editDescFormRef = ref();

const restoreVisible = ref(false);
const restoreRow = ref<any>(null);

const search = async () => {
    loading.value = true;
    try {
        const res = await searchFullBackup({
            page: paginationConfig.currentPage,
            pageSize: paginationConfig.pageSize,
            orderBy: 'createdAt',
            order: 'descending',
        });
        data.value = res.items || [];
        paginationConfig.total = res.total || 0;
    } catch (e) {
        console.error(e);
    } finally {
        loading.value = false;
    }
};

const onCreate = () => {
    createForm.description = '';
    createVisible.value = true;
};

const submitCreate = async () => {
    await createFullBackup({ description: createForm.description });
    createVisible.value = false;
    ElMessage.success(i18n.global.t('commons.msg.createSuccess'));
    await search();
};

const onEditDesc = (row: any) => {
    editDescForm.id = row.id;
    editDescForm.description = row.description || '';
    editDescVisible.value = true;
};

const submitEditDesc = async () => {
    await updateFullBackupDescription(editDescForm.id, editDescForm.description);
    editDescVisible.value = false;
    ElMessage.success(i18n.global.t('commons.msg.updateSuccess'));
    await search();
};

const onRestore = (row: any) => {
    restoreRow.value = row;
    restoreVisible.value = true;
};

const submitRestore = async () => {
    if (!restoreRow.value) return;
    await restoreFullBackup({ id: restoreRow.value.id });
    restoreVisible.value = false;
    ElMessage.success(i18n.global.t('commons.msg.restoreSuccess'));
};

const onDelete = async (row: any) => {
    await deleteFullBackup({ ids: [row.id], deleteWithFile: true });
    ElMessage.success(i18n.global.t('commons.msg.deleteSuccess'));
    await search();
};

const onBatchDelete = async () => {
    const ids = selects.value.map((s: any) => s.id);
    await deleteFullBackup({ ids, deleteWithFile: true });
    selects.value = [];
    ElMessage.success(i18n.global.t('commons.msg.deleteSuccess'));
    await search();
};

onMounted(() => {
    search();
});
</script>