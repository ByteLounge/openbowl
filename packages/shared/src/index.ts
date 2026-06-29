/**
 * OpenBowl Common Utilities
 */

/**
 * Estimates token count based on a 4-character-per-token rough ratio.
 * Useful for client-side budget indicators before receiving final statistics.
 */
export function approximateTokens(text: string): number {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

/**
 * Formats token usage costs into a human-readable string.
 */
export function formatCost(amount: number): string {
  if (amount === 0) return '$0.00';
  if (amount < 0.01) return `$${amount.toFixed(5)}`;
  return `$${amount.toFixed(2)}`;
}

/**
 * Formats iso string to localized date
 */
export function formatDate(isoString: string): string {
  try {
    const d = new Date(isoString);
    return d.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return isoString;
  }
}
