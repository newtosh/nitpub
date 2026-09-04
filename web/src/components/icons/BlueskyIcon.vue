<script setup lang="ts">
/** Simplified Bluesky butterfly mark — hand-drawn approximation, not a
 * pixel-perfect brand asset, just needs to read as "not Mastodon." */
withDefaults(
  defineProps<{
    size?: number
  }>(),
  { size: 12 },
)
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    role="img"
    aria-hidden="true"
    class="bluesky-icon"
  >
    <path
      fill="currentColor"
      d="M12 9.5C10.4 6.2 6.7 2.4 3.9 1.2 1.9.4 1 1.1 1 2.7c0 1.6.7 5.6 1.2 6.9.9 2.3 3.5 3.1 5.8 2.7-3.3.5-6.3 1.8-2.3 6.2 4.2 4.6 5.9-1 6.3-2.9.4 1.9 2 7.5 6.3 2.9 4-4.4 1-5.7-2.3-6.2 2.3.4 4.9-.4 5.8-2.7.5-1.3 1.2-5.3 1.2-6.9 0-1.6-.9-2.3-2.9-1.5-2.8 1.2-6.5 5-8.1 8.3Z"
    />
  </svg>
</template>

<style scoped>
.bluesky-icon {
  display: block;
  flex-shrink: 0;
}
</style>
