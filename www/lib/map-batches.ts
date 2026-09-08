/** Apply bounded batches without starting an unbounded number of requests. */
export async function mapBatches<T, R>(
  items: T[],
  load: (batch: T[]) => Promise<R>,
  batchSize = 200,
  concurrency = 4,
): Promise<R[]> {
  const results: R[] = []
  for (let start = 0; start < items.length; start += batchSize * concurrency) {
    const pending: Promise<R>[] = []
    for (let offset = start; offset < Math.min(items.length, start + batchSize * concurrency); offset += batchSize) {
      pending.push(load(items.slice(offset, offset + batchSize)))
    }
    results.push(...(await Promise.all(pending)))
  }
  return results
}
