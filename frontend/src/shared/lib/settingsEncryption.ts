export function maskSensitiveValue(value: string, maskLength: number = 4): string {
  if (!value || value.length <= maskLength) {
    return '****';
  }
  const start = value.slice(0, maskLength);
  return `${start}****`;
}
