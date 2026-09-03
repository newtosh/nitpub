<script setup lang="ts">
// Write/Preview tab bar for an editor embedded in a larger form with its
// own combined preview (e.g. ComposeForm's article/quote sections) —
// reuses MarkdownEditor's own .md-editor-tabs markup/styling so the two
// stay visually identical without copy-pasting the tab buttons.
const model = defineModel<'write' | 'preview'>({ required: true })
</script>

<template>
  <div class="md-editor-tabs">
    <button type="button" :class="{ active: model === 'write' }" title="Write" aria-label="Write" @click="model = 'write'">
      Write
    </button>
    <button type="button" :class="{ active: model === 'preview' }" title="Preview" aria-label="Preview" @click="model = 'preview'">
      Preview
    </button>
  </div>
</template>
