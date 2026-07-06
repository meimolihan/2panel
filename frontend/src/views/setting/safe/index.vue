<template>
    <div>
        <LayoutContent v-loading="loading" :title="$t('setting.safe')" :divider="true">
            <template #main>
                <el-form
                    :model="form"
                    v-loading="loading"
                    :label-position="isMobile ? 'top' : 'left'"
                    label-width="150px"
                >
                    <el-row>
                        <el-col :span="1"><br /></el-col>
                        <el-col :xs="24" :sm="20" :md="15" :lg="12" :xl="12">
                            <el-form-item :label="$t('setting.panelPort')" prop="serverPort">
                                <el-input disabled v-model.number="form.serverPort">
                                    <template #append>
                                        <el-button @click="onChangePort" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item :label="$t('setting.bindInfo')" prop="bindAddress">
                                <el-input disabled v-model="form.bindAddress">
                                    <template #append>
                                        <el-button @click="onChangeBind" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item :label="$t('setting.entrance')">
                                <el-input
                                    type="password"
                                    disabled
                                    v-if="form.securityEntrance"
                                    v-model="form.securityEntrance"
                                >
                                    <template #append>
                                        <el-button @click="onChangeEntrance" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <el-input disabled v-if="!form.securityEntrance" v-model="unset">
                                    <template #append>
                                        <el-button @click="onChangeEntrance" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <span class="input-help">{{ $t('setting.entranceHelper') }}</span>
                            </el-form-item>

                            <el-form-item :label="$t('setting.noAuthSetting')">
                                <el-input disabled v-model="form.noAuthSetting">
                                    <template #append>
                                        <el-button @click="onChangeResponse" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                            </el-form-item>

                            <el-form-item :label="$t('setting.allowIPs')">
                                <div style="width: 100%" v-if="form.allowIPs">
                                    <el-input
                                        type="textarea"
                                        :rows="3"
                                        disabled
                                        v-model="form.allowIPs"
                                        style="width: calc(100% - 80px)"
                                    />
                                    <el-button class="append-button" @click="onChangeAllowIPs" icon="Setting">
                                        {{ $t('commons.button.set') }}
                                    </el-button>
                                </div>
                                <el-input disabled v-if="!form.allowIPs" v-model="unset">
                                    <template #append>
                                        <el-button @click="onChangeAllowIPs" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <span class="input-help">{{ $t('setting.allowIPsHelper') }}</span>
                            </el-form-item>

                            <el-form-item :label="$t('setting.bindDomain')">
                                <el-input disabled v-if="form.bindDomain" v-model="form.bindDomain">
                                    <template #append>
                                        <el-button @click="onChangeBindDomain" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <el-input disabled v-if="!form.bindDomain" v-model="unset">
                                    <template #append>
                                        <el-button @click="onChangeBindDomain" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <span class="input-help">{{ $t('setting.bindDomainHelper') }}</span>
                            </el-form-item>


                            <el-form-item :label="$t('setting.expirationTime')" prop="expirationDays">
                                <el-input disabled v-model="form.expirationDays">
                                    <template #append>
                                        <el-button @click="onChangeExpirationDays" icon="Setting">
                                            {{ $t('commons.button.set') }}
                                        </el-button>
                                    </template>
                                </el-input>
                                <span class="input-help">
                                    {{
                                        form.expirationDays === 0
                                            ? $t('setting.noneSetting')
                                            : $t('setting.expirationHelper')
                                    }}
                                </span>
                            </el-form-item>
                            <el-form-item :label="$t('setting.complexity')" prop="complexityVerification">
                                <el-switch
                                    @change="onSaveComplexity"
                                    v-model="form.complexityVerification"
                                    active-value="Enable"
                                    inactive-value="Disable"
                                />
                                <span class="input-help">
                                    {{ $t('setting.complexityHelper') }}
                                </span>
                            </el-form-item>
                        </el-col>
                    </el-row>
                </el-form>
            </template>
        </LayoutContent>

        <PortSetting ref="portRef" />
        <BindSetting ref="bindRef" />
        <EntranceSetting ref="entranceRef" @search="search" />
        <DomainSetting ref="domainRef" @search="search" />
        <AllowIPsSetting ref="allowIPsRef" @search="search" />
        <ExpirationSetting ref="expirationRef" @search="search" />
        <ResponseSetting ref="responseRef" @search="search()" />
    </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue';
import { ElForm, ElMessageBox } from 'element-plus';
import PortSetting from '@/views/setting/safe/port/index.vue';
import BindSetting from '@/views/setting/safe/bind/index.vue';
import ResponseSetting from '@/views/setting/safe/response/index.vue';
import ExpirationSetting from '@/views/setting/safe/expiration/index.vue';
import EntranceSetting from '@/views/setting/safe/entrance/index.vue';
import DomainSetting from '@/views/setting/safe/domain/index.vue';
import AllowIPsSetting from '@/views/setting/safe/allowips/index.vue';
import { updateSetting, getSettingInfo, getSystemAvailable } from '@/api/modules/setting';
import i18n from '@/lang';
import { MsgSuccess } from '@/utils/message';
import { useGlobalStore } from '@/composables/useGlobalStore';

const { entrance, isLogin, isMobile } = useGlobalStore();

const loading = ref(false);
const entranceRef = ref();
const portRef = ref();
const bindRef = ref();
const expirationRef = ref();
const responseRef = ref();

const domainRef = ref();
const allowIPsRef = ref();

const form = reactive({
    serverPort: 9999,
    ipv6: 'Disable',
    bindAddress: '',

    securityEntrance: '',
    expirationDays: 0,
    complexityVerification: 'Disable',
    allowIPs: '',
    bindDomain: '',
    noAuthSetting: '200 - ' + i18n.global.t('setting.help200'),
    noAuthSettingValue: '200',
});

const unset = ref(i18n.global.t('setting.unSetting'));

const search = async () => {
    const res = await getSettingInfo();
    form.serverPort = Number(res.data.serverPort);
    form.ipv6 = res.data.ipv6;
    form.bindAddress = res.data.bindAddress;

    form.securityEntrance = res.data.securityEntrance;
    form.expirationDays = Number(res.data.expirationDays);
    form.complexityVerification = res.data.complexityVerification;
    form.allowIPs = res.data.allowIPs.replaceAll(',', '\n');
    form.bindDomain = res.data.bindDomain;
    form.noAuthSettingValue = res.data.noAuthSetting;
    if (res.data.noAuthSetting !== '200') {
        form.noAuthSetting = res.data.noAuthSetting + ' - ' + i18n.global.t('setting.error' + res.data.noAuthSetting);
    } else {
        form.noAuthSetting = res.data.noAuthSetting + ' - ' + i18n.global.t('setting.help200');
    }
};

const onSaveComplexity = async () => {
    let param = {
        key: 'ComplexityVerification',
        value: form.complexityVerification,
    };
    loading.value = true;
    await updateSetting(param)
        .then(() => {
            loading.value = false;
            MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
            search();
        })
        .catch(() => {
            loading.value = false;
        });
};

const onChangeEntrance = () => {
    entranceRef.value.acceptParams({ securityEntrance: form.securityEntrance });
};
const onChangePort = () => {
    portRef.value.acceptParams({ serverPort: form.serverPort });
};
const onChangeBind = () => {
    bindRef.value.acceptParams({ ipv6: form.ipv6, bindAddress: form.bindAddress });
};
const onChangeResponse = () => {
    responseRef.value.acceptParams({ noAuthSetting: form.noAuthSettingValue });
};
const onChangeBindDomain = () => {
    domainRef.value.acceptParams({ bindDomain: form.bindDomain });
};
const onChangeAllowIPs = () => {
    allowIPsRef.value.acceptParams({ allowIPs: form.allowIPs });
};
const onChangeExpirationDays = async () => {
    expirationRef.value.acceptParams({ expirationDays: form.expirationDays });
};
onMounted(() => {
    search();
    getSystemAvailable();
});
</script>

<style lang="scss" scoped>
.append-button {
    width: 80px;
    background-color: var(--el-fill-color-light);
    color: var(--el-color-info);
}
</style>
