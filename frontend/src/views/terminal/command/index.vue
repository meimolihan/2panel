<template>
    <div>
        <LayoutContent v-loading="loading" :title="$t('terminal.quickCommand')">
            <template #prompt>
                <el-alert type="info" :title="$t('terminal.quickCommandHelper')" :closable="false" />
            </template>
            <template #leftToolBar>
                <el-button type="primary" @click="onOpenDialog('create')">
                    {{ $t('commons.button.create') }}
                </el-button>
                <el-button type="primary" plain :disabled="selects.length === 0" @click="batchDelete(null)">
                    {{ $t('commons.button.delete') }}
                </el-button>

                <el-button-group>
                    <el-button @click="onImport">
                        {{ $t('commons.button.import') }}
                    </el-button>
                    <el-button @click="onExport">
                        {{ $t('commons.button.export') }}
                    </el-button>
                </el-button-group>
            </template>
            <template #rightToolBar>
                <TableSearch @search="search()" v-model:searchName="info" />
                <TableRefresh @search="search()" />
            </template>
            <template #main>
                <ComplexTable
                    :pagination-config="paginationConfig"
                    v-model:selects="selects"
                    :data="data"
                    @sort-change="search"
                    @search="search"
                    :heightDiff="350"
                >
                    <el-table-column type="selection" fix />
                    <el-table-column
                        :label="$t('commons.table.name')"
                        show-overflow-tooltip
                        min-width="100"
                        prop="name"
                        fix
                        sortable
                    />
                    <el-table-column
                        :label="$t('terminal.command')"
                        min-width="300"
                        show-overflow-tooltip
                        prop="command"
                        sortable
                    />

                    <fu-table-operations width="200px" :buttons="buttons" :label="$t('commons.table.operate')" fix />
                </ComplexTable>
            </template>
        </LayoutContent>

        <OpDialog ref="opRef" @search="search" />
        <OperateDialog @search="search" ref="dialogRef" />
        <ImportDialog @search="search" ref="importDialogRef" />
    </div>
</template>

<script setup lang="ts">
import { Command } from '@/api/interface/command';
import OperateDialog from '@/views/terminal/command/operate/index.vue';
import ImportDialog from '@/views/terminal/command/import/index.vue';
import { editCommand, deleteCommand, getCommandPage, exportCommands } from '@/api/modules/command';
import { reactive, ref } from 'vue';
import i18n from '@/lang';
import { MsgSuccess } from '@/utils/message';
import { downloadFile } from '@/utils/file';
const loading = ref();
const data = ref();
const selects = ref<any>([]);
const paginationConfig = reactive({
    cacheSizeKey: 'terminal-command-page-size',
    currentPage: 1,
    pageSize: Number(localStorage.getItem('terminal-command-page-size')) || 20,
    total: 0,
    orderBy: 'name',
    order: 'ascending',
});
const info = ref();
const dialogRef = ref();
const opRef = ref();
const importDialogRef = ref();

const acceptParams = () => {
    search();
};

const onOpenDialog = async (
    title: string,
    rowData: Partial<Command.CommandInfo> = {
        type: 'command',
    },
) => {
    let params = {
        title,
        rowData: { ...rowData },
    };
    dialogRef.value!.acceptParams(params);
};

const onExport = async () => {
    loading.value = true;
    await exportCommands()
        .then((res) => {
            if (res.data) {
                loading.value = false;
                downloadFile(res.data, 'local');
            }
        })
        .catch(() => {
            loading.value = false;
        });
};

const onImport = () => {
    importDialogRef.value.acceptParams();
};

const batchDelete = async (row: Command.CommandInfo | null) => {
    let names = [];
    let ids = [];
    if (row) {
        ids = [row.id];
        names = [row.name];
    } else {
        selects.value.forEach((item: Command.CommandInfo) => {
            ids.push(item.id);
            names.push(item.name);
        });
    }
    opRef.value.acceptParams({
        title: i18n.global.t('commons.button.delete'),
        names: names,
        msg: i18n.global.t('commons.msg.operatorHelper', [
            i18n.global.t('terminal.quickCommand'),
            i18n.global.t('commons.button.delete'),
        ]),
        api: deleteCommand,
        params: { ids: ids },
    });
};

const buttons = [
    {
        label: i18n.global.t('commons.button.edit'),
        click: (row: any) => {
            onOpenDialog('edit', row);
        },
    },
    {
        label: i18n.global.t('commons.button.delete'),
        click: batchDelete,
    },
];

const search = async (column?: any) => {
    paginationConfig.orderBy = column?.order ? column.prop : paginationConfig.orderBy;
    paginationConfig.order = column?.order ? column.order : paginationConfig.order;
    let params = {
        page: paginationConfig.currentPage,
        pageSize: paginationConfig.pageSize,
        info: info.value,
        orderBy: paginationConfig.orderBy,
        order: paginationConfig.order,
        type: 'command',
    };
    loading.value = true;
    await getCommandPage(params)
        .then((res) => {
            loading.value = false;
            data.value = res.data.items || [];
            paginationConfig.total = res.data.total;
        })
        .catch(() => {
            loading.value = false;
        });
};

defineExpose({
    acceptParams,
});
</script>
