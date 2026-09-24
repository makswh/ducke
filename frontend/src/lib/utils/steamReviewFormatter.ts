/**
 * Utility functions for parsing and rendering Steam user reviews BBCode cleanly and safely
 */

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

export function formatSteamReviewBBCode(raw: string): string {
  if (!raw) return '';

  let text = escapeHtml(raw.trim());

  // Headings
  text = text.replace(/\[h1\]([\s\S]*?)\[\/h1\]/gi, '<div class="text-white font-bold text-xs uppercase tracking-wide mt-2 mb-1">$1</div>');
  text = text.replace(/\[h2\]([\s\S]*?)\[\/h2\]/gi, '<div class="text-white font-semibold text-xs mt-2 mb-1">$1</div>');
  text = text.replace(/\[h3\]([\s\S]*?)\[\/h3\]/gi, '<div class="text-white font-medium text-xs mt-1.5 mb-1">$1</div>');

  // Basic formatting
  text = text.replace(/\[b\]([\s\S]*?)\[\/b\]/gi, '<strong class="text-white font-semibold">$1</strong>');
  text = text.replace(/\[i\]([\s\S]*?)\[\/i\]/gi, '<em class="italic text-[#cbd5e1]">$1</em>');
  text = text.replace(/\[u\]([\s\S]*?)\[\/u\]/gi, '<span class="underline underline-offset-2">$1</span>');
  text = text.replace(/\[strike\]([\s\S]*?)\[\/strike\]/gi, '<span class="line-through text-[#64748b]">$1</span>');

  // Spoilers (interactive hidden block)
  text = text.replace(
    /\[spoiler\]([\s\S]*?)\[\/spoiler\]/gi,
    '<span class="bg-[#1e232d] text-transparent hover:text-white rounded px-1 transition-colors duration-150 cursor-pointer select-none hover:select-text" title="Спойлер (наведите для просмотра)">$1</span>'
  );

  // Quotes
  text = text.replace(
    /\[quote(?:=[^\]]*)?\]([\s\S]*?)\[\/quote\]/gi,
    '<blockquote class="border-l-2 border-white/20 bg-white/[0.02] pl-3 py-1 my-1.5 text-[#94a3b8] italic rounded-r text-xs">$1</blockquote>'
  );

  // Lists
  text = text.replace(/\[list\]([\s\S]*?)\[\/list\]/gi, '<ul class="list-disc pl-4 space-y-0.5 my-1.5 text-xs">$1</ul>');
  text = text.replace(/\[\*\](.*?)(\n|\[\*\]|\[\/list\]|$)/gi, '<li>$1</li>');

  // Links (neutralized safely without opening arbitrary third party pages)
  text = text.replace(/\[url=[^\]]*\]([\s\S]*?)\[\/url\]/gi, '<span class="text-sky-400 font-medium">$1</span>');
  text = text.replace(/\[url\]([\s\S]*?)\[\/url\]/gi, '<span class="text-sky-400 font-medium">$1</span>');

  // Line breaks
  text = text.replace(/\r\n/g, '<br>').replace(/\n/g, '<br>');

  // Clean redundant triple line breaks
  text = text.replace(/(<br>\s*){3,}/g, '<br><br>');

  return text;
}

export function formatReviewDate(unixSeconds: number): string {
  if (!unixSeconds || unixSeconds <= 0) return '';
  try {
    const date = new Date(unixSeconds * 1000);
    return date.toLocaleDateString('ru-RU', {
      day: 'numeric',
      month: 'short',
      year: 'numeric'
    });
  } catch {
    return '';
  }
}
