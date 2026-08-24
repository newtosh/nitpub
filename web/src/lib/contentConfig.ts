export function externalLinksNewTabFromConfig(
  content?: { external_links_new_tab?: boolean | null },
): boolean {
  if (content?.external_links_new_tab === undefined || content.external_links_new_tab === null) {
    return true
  }
  return content.external_links_new_tab
}
