export function matchesPortSearch(query: string, ...ports: (number | undefined)[]): boolean {
  const search = query.trim()
  return search === '' || ports.some((port) => port !== undefined && String(port).includes(search))
}
