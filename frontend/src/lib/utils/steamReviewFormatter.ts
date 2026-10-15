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
  text = text.replace(/\[h1\]([\s\S]*?)\[\/h1\]/gi, '<div class="text-white font-bold text-sm tracking-wide mt-3 mb-1.5">$1</div>');
  text = text.replace(/\[h2\]([\s\S]*?)\[\/h2\]/gi, '<div class="text-white font-semibold text-[13px] mt-2.5 mb-1">$1</div>');
  text = text.replace(/\[h3\]([\s\S]*?)\[\/h3\]/gi, '<div class="text-white font-medium text-xs mt-2 mb-1">$1</div>');

  // Basic formatting
  text = text.replace(/\[b\]([\s\S]*?)\[\/b\]/gi, '<strong class="text-white font-semibold">$1</strong>');
  text = text.replace(/\[i\]([\s\S]*?)\[\/i\]/gi, '<em class="italic text-[#cbd5e1]">$1</em>');
  text = text.replace(/\[u\]([\s\S]*?)\[\/u\]/gi, '<span class="underline underline-offset-2">$1</span>');
  text = text.replace(/\[strike\]([\s\S]*?)\[\/strike\]/gi, '<span class="line-through text-[#64748b]">$1</span>');

  // Spoilers (interactive hidden block)
  text = text.replace(
    /\[spoiler\]([\s\S]*?)\[\/spoiler\]/gi,
    '<span class="bg-[#1c222d] hover:bg-[#252e3d] text-transparent hover:text-white rounded-[4px] px-1.5 py-0.5 border border-white/10 transition-colors duration-150 cursor-pointer select-none hover:select-text" title="Спойлер (наведите для просмотра)">$1</span>'
  );

  // Quotes
  text = text.replace(
    /\[quote(?:=[^\]]*)?\]([\s\S]*?)\[\/quote\]/gi,
    '<blockquote class="border-l-2 border-sky-400/40 bg-white/[0.03] pl-3 py-1.5 my-2 text-[#94a3b8] italic rounded-r text-[12px] leading-relaxed">$1</blockquote>'
  );

  // Lists
  text = text.replace(/\[list\]([\s\S]*?)\[\/list\]/gi, '<ul class="list-disc pl-5 space-y-1 my-2 text-[12px] text-slate-300">$1</ul>');
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
      month: 'long',
      year: 'numeric'
    });
  } catch {
    return '';
  }
}
