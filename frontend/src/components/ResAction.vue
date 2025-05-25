<template>
  <NSpace style="--wails-draggable:no-drag" :size="2">
    <NButton v-if="(row.Classify != 'live' && row.Classify != 'm3u8') && row.Status !== 'done'" type="success" :tertiary="true" size="tiny" @click="action('down')">
      <template #icon>
        <DownloadOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("index.direct_download") }}</span>
    </NButton>
    <NButton type="info" :tertiary="true" size="tiny" @click="action('copy')">
      <template #icon>
        <LinkOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("index.copy_link") }}</span>
    </NButton>
    <NButton v-if="row.Classify != 'live' && row.Classify != 'm3u8'" type="info" :tertiary="true" size="tiny" @click="action('open')">
      <template #icon>
        <GlobalOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("index.open_link") }}</span>
    </NButton>
    <NButton v-if="row.DecodeKey && false" type="warning" :tertiary="true" size="tiny" @click="action('decode')">
      <template #icon>
        <UnlockOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("index.video_decode") }}</span>
    </NButton>
    <NButton type="info" :tertiary="true" size="tiny" @click="action('json')">
      <template #icon>
        <CopyOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("index.copy_data") }}</span>
    </NButton>
    <NButton v-if="row.SavePath && row.Status === 'done'" type="info" :tertiary="true" size="tiny" @click="action('folder')">
      <template #icon>
        <FolderOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ row.SavePath }}</span>
    </NButton>
    <NButton type="error" :tertiary="true" size="tiny" @click="action('delete')">
      <template #icon>
        <DeleteOutlined style="font-size: 14px" />
      </template>
      <span class="action-tooltip">{{ t("common.delete") }}</span>
    </NButton>
  </NSpace>
</template>

<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {
  DownloadOutlined,
  LinkOutlined,
  GlobalOutlined,
  UnlockOutlined,
  CopyOutlined,
  DeleteOutlined,
  FolderOutlined
} from '@vicons/antd'

const {t} = useI18n()
const props = defineProps<{
  row: any,
  index: number,
}>()

const emits = defineEmits(["action"])

const action = (type: string) => {
  emits('action', props.row, props.index, type)
}
</script>

<style scoped>
.action-tooltip {
  display: none;
  position: absolute;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  white-space: nowrap;
  z-index: 1000;
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.n-button:hover .action-tooltip {
  display: block;
}

:deep(.n-button) {
  padding: 0 6px;
  height: 24px;
}

:deep(.n-button .n-button__icon) {
  margin-right: 0;
}

:deep(.n-button--disabled) {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>