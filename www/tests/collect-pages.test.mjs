import test from 'node:test'
import assert from 'node:assert/strict'
import { collectPages } from '../lib/collect-pages.ts'
test('collects a full catalog in bounded pages without truncation', async () => {
  const calls = []
  const result = await collectPages(async (page, size) => {
    calls.push([page, size])
    return {
      total: 425,
      items: Array.from({ length: Math.min(size, 425 - (page - 1) * size) }, (_, i) => (page - 1) * size + i),
    }
  })
  assert.equal(result.items.length, 425)
  assert.equal(result.items[424], 424)
  assert.deepEqual(calls, [
    [1, 200],
    [2, 200],
    [3, 200],
  ])
})
test('does not poll endlessly when a catalog changes under pagination', async () => {
  await assert.rejects(
    collectPages(async () => ({ total: 10, items: [] })),
    /刷新/,
  )
})
import { mapBatches } from '../lib/map-batches.ts'
test('limits status query payloads and concurrent requests', async () => {
  let active = 0,
    peak = 0
  const result = await mapBatches(
    Array.from({ length: 1825 }, (_, i) => i),
    async (batch) => {
      assert(batch.length <= 200)
      active++
      peak = Math.max(peak, active)
      await new Promise((resolve) => setTimeout(resolve, 1))
      active--
      return batch
    },
  )
  assert(peak <= 4)
  assert.equal(result.flat().length, 1825)
  assert.equal(result.flat()[1824], 1824)
})
