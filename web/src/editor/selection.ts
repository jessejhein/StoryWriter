export function countWords(text: string): number {
  const trimmed = text.trim()
  if (!trimmed) {
    return 0
  }
  return trimmed.split(/\s+/u).filter(Boolean).length
}

export function toUTF8ByteOffset(text: string, charOffset: number): number {
  return new TextEncoder().encode(text.slice(0, charOffset)).length
}

export function toUTF8ByteRange(text: string, start: number, end: number): { startByte: number; endByte: number } {
  return {
    startByte: toUTF8ByteOffset(text, start),
    endByte: toUTF8ByteOffset(text, end),
  }
}
