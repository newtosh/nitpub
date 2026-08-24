import type { Component } from 'vue'
import {
  Book,
  BookOpen,
  Briefcase,
  Code,
  ExternalLink,
  FileText,
  Folder,
  Globe,
  Home,
  Info,
  Link,
  Link2,
  Mail,
  Newspaper,
  Rss,
  Search,
  User,
} from '@lucide/vue'

const iconMap: Record<string, Component> = {
  user: User,
  newspaper: Newspaper,
  link: Link,
  links: Link2,
  folder: Folder,
  book: Book,
  'book-open': BookOpen,
  home: Home,
  rss: Rss,
  github: Code,
  globe: Globe,
  mail: Mail,
  info: Info,
  'file-text': FileText,
  briefcase: Briefcase,
  code: Code,
  search: Search,
  'external-link': ExternalLink,
}

export const allowedIcons = Object.keys(iconMap)

export function navIcon(name?: string): Component | null {
  if (!name) return null
  return iconMap[name] ?? null
}
