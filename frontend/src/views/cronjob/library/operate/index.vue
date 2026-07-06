<template>
    <DrawerPro
        v-model="drawerVisible"
        :header="title"
        @close="handleClose"
        :resource="dialogData.title !== 'edit' ? '' : dialogData.rowData?.name"
        size="large"
        :autoClose="false"
        :fullScreen="true"
    >
        <el-form ref="formRef" v-loading="loading" label-position="top" :model="dialogData.rowData" :rules="rules">
            <el-form-item :label="$t('commons.table.name')" prop="name">
                <el-tag v-if="dialogData.title === 'edit'">{{ dialogData.rowData!.name }}</el-tag>
                <el-input v-else v-model="dialogData.rowData!.name" />
            </el-form-item>
            <el-form-item prop="isInteractive">
                <el-checkbox v-model="dialogData.rowData!.isInteractive">
                    {{ $t('cronjob.library.interactive') }}
                </el-checkbox>
                <span class="input-help">{{ $t('cronjob.library.interactiveHelper') }}</span>
            </el-form-item>

            <el-form-item :label="$t('cronjob.shellContent')" prop="script" class="mt-5">
                <CodemirrorPro
                    v-model="dialogData.rowData!.script"
                    placeholder="#Define or paste the content of your script file here"
                    mode="javascript"
                    :heightDiff="400"
                />
            </el-form-item>
            <el-form-item :label="$t('commons.table.description')" prop="description">
                <el-input clearable v-model="dialogData.rowData!.description" />
            </el-form-item>
        </el-form>
        <template #footer>
            <span class="dialog-footer">
                <el-button @click="drawerVisible = false">{{ $t('commons.button.cancel') }}</el-button>
                <el-button type="primary" @click="onSubmit(formRef)">
                    {{ $t('commons.button.confirm') }}
                </el-button>
            </span>
        </template>
    </DrawerPro>
</template>

<script lang="ts" setup>
import { reactive, ref } from 'vue';
import i18n from '@/lang';
import { ElForm } from 'element-plus';
import { Cronjob } from '@/api/interface/cronjob';
import { MsgSuccess } from '@/utils/message';
import { Rules } from '@/global/form-rules';
import { addScript, editScript } from '@/api/modules/cronjob';

interface DialogProps {
    title: string;
    rowData?: Cronjob.ScriptInfo;
    getTableList?: () => Promise<any>;
}
const title = ref<string>('');
const drawerVisible = ref(false);
const dialogData = ref<DialogProps>({
    title: '',
});
const loading = ref();

const acceptParams = (params: DialogProps): void => {
    dialogData.value = params;
    title.value = i18n.global.t('commons.button.' + dialogData.value.title);
    drawerVisible.value = true;
};
const emit = defineEmits<{ (e: 'search'): void }>();

const handleClose = () => {
    drawerVisible.value = false;
};

const rules = reactive({
    name: [Rules.requiredInput],
    script: [Rules.requiredInput],
});

type FormInstance = InstanceType<typeof ElForm>;
const formRef = ref<FormInstance>();

const onSubmit = async (formEl: FormInstance | undefined) => {
    if (!formEl) return;
    formEl.validate(async (valid) => {
        if (!valid) return;
        loading.value = true;
        if (dialogData.value.title === 'create' || dialogData.value.title === 'clone') {
            await addScript(dialogData.value.rowData)
                .then(() => {
                    loading.value = false;
                    MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
                    emit('search');
                    drawerVisible.value = false;
                })
                .catch(() => {
                    loading.value = false;
                });
            return;
        }

        await editScript(dialogData.value.rowData)
            .then(() => {
                loading.value = false;
                MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
                emit('search');
                drawerVisible.value = false;
            })
            .catch(() => {
                loading.value = false;
            });
    });
};

defineExpose({
    acceptParams,
});
</script>
