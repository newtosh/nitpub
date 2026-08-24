export type PublicReply = {
  actor: string
  author_name?: string
  content: string
  url?: string
  avatar_url?: string
  object_id?: string
  parent_object_id?: string
  received_at?: string
}

export async function fetchReplies(slug: string): Promise<PublicReply[]> {
  const res = await fetch(`/api/posts/${slug}/replies`)
  if (!res.ok) throw new Error('Failed to load replies')
  return res.json()
}

export type ReplyNode = {
  reply: PublicReply
  children: ReplyNode[]
}

/** Reconstructs reply-to-reply threading from the flat, chronologically
 * ordered list the API returns: a reply nests under another reply in the
 * same list when its parent_object_id matches that reply's object_id.
 * A reply whose parent isn't in the list (top-level, or its parent was
 * never approved/seen) surfaces as a root — never dropped. */
export function buildReplyTree(replies: PublicReply[]): ReplyNode[] {
  const byObjectID = new Map<string, ReplyNode>()
  for (const reply of replies) {
    if (reply.object_id) byObjectID.set(reply.object_id, { reply, children: [] })
  }

  const roots: ReplyNode[] = []
  for (const reply of replies) {
    const node = reply.object_id ? byObjectID.get(reply.object_id)! : { reply, children: [] }
    const parent = reply.parent_object_id ? byObjectID.get(reply.parent_object_id) : undefined
    if (parent && parent !== node) {
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  }
  return roots
}

/** Total reply count across an entire thread tree, including nested replies. */
export function countReplyTree(nodes: ReplyNode[]): number {
  let n = 0
  for (const node of nodes) n += 1 + countReplyTree(node.children)
  return n
}
