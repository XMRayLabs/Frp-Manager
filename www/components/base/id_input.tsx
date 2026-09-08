import { Input } from '@/components/ui/input'
import { Search, X } from 'lucide-react'
import { useEffect, useState } from 'react'
export interface IdInputProps {
  setKeyword: (keyword: string) => void
  keyword: string
  refetchTrigger?: (key: string) => void
}
export function IdInput({ setKeyword, keyword, refetchTrigger }: IdInputProps) {
  const [input, setInput] = useState(keyword)
  useEffect(() => setInput(keyword), [keyword])
  useEffect(() => {
    const timer = setTimeout(() => {
      if (input !== keyword) setKeyword(input)
    }, 350)
    return () => clearTimeout(timer)
  }, [input, keyword, setKeyword])
  return (
    <form
      className="relative w-full max-w-sm"
      onSubmit={(event) => {
        event.preventDefault()
        setKeyword(input)
        refetchTrigger?.(String(Date.now()))
      }}
    >
      <Search className="absolute left-3 top-3 size-4 text-muted-foreground" />
      <Input
        aria-label="搜索节点"
        className="pl-9 pr-9"
        placeholder="搜索节点 ID…"
        value={input}
        onChange={(event) => setInput(event.target.value)}
      />
      {input && (
        <button
          type="button"
          aria-label="清空搜索"
          className="absolute right-3 top-3"
          onClick={() => {
            setInput('')
            setKeyword('')
          }}
        >
          <X className="size-4" />
        </button>
      )}
    </form>
  )
}
