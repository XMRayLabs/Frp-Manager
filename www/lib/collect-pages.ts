/** Full-fleet filters use bounded pages instead of a silently truncated 10,000-row request. */
export async function collectPages<T>(
  load: (page: number, pageSize: number) => Promise<{ total: number; items: T[] }>,
) {
  const items: T[] = []
  let total = 0
  for (let page = 1; ; page++) {
    const result = await load(page, 200)
    total = result.total
    items.push(...result.items)
    if (items.length >= total) return { total, items }
    if (result.items.length === 0) throw new Error('节点列表在加载中发生变化，请刷新后重试。')
  }
}
