let counter = 0;

export function randomUUID(): string {
  counter += 1;
  return `00000000-0000-4000-8000-${counter.toString().padStart(12, '0')}`;
}

export function __resetCounter(): void {
  counter = 0;
}
