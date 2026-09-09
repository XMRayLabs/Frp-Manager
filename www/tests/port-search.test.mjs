import test from 'node:test'
import assert from 'node:assert/strict'
import { matchesPortSearch } from '../lib/port-search.ts'

test('port search matches substrings in either port, not a numeric range', () => {
  assert(matchesPortSearch('6000', 80, 6000))
  assert(matchesPortSearch('6000', 16000, 80))
  assert(matchesPortSearch('6000', 80, 60001))
  assert(!matchesPortSearch('6000', 6001, 65000))
  assert(!matchesPortSearch('6000', undefined, undefined))
  assert(matchesPortSearch('', undefined, undefined))
  assert(matchesPortSearch(' 6000 ', 6000, undefined))
})
