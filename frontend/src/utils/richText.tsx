import { Fragment, type ReactNode } from 'react';
import { Box, Typography } from '@mui/material';

/**
 * Minimal, safe rich-text renderer for task descriptions. Supports a small
 * Markdown subset built into React nodes (no dangerouslySetInnerHTML):
 *   **bold**, *italic* / _italic_, `code`, http(s) links, "- " / "* " bullet
 *   lists, blank lines.
 */

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  // Order matters: URLs are matched before the emphasis markers so an
  // underscore inside a link isn't treated as italics.
  const tokens = text.split(/(https?:\/\/[^\s]+|\*\*[^*]+\*\*|\*[^*\n]+\*|_[^_\n]+_|`[^`\n]+`)/g);
  return tokens.filter((t) => t !== '').map((tok, i) => {
    const key = `${keyPrefix}-${i}`;
    if (/^https?:\/\//i.test(tok)) {
      // Trailing sentence punctuation shouldn't be part of the link.
      const m = tok.match(/^(https?:\/\/[^\s]*?)([.,;:!?)\]]*)$/i);
      const href = m ? m[1] : tok;
      const trail = m ? m[2] : '';
      return (
        <Fragment key={key}>
          <Box
            component="a"
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            sx={{ color: 'primary.main', wordBreak: 'break-all' }}
          >
            {href}
          </Box>
          {trail}
        </Fragment>
      );
    }
    if (tok.startsWith('**') && tok.endsWith('**')) {
      return <strong key={key}>{tok.slice(2, -2)}</strong>;
    }
    if (tok.startsWith('`') && tok.endsWith('`')) {
      return (
        <Box
          component="code"
          key={key}
          sx={{ px: 0.5, borderRadius: 0.5, bgcolor: 'action.hover', fontFamily: 'monospace', fontSize: '0.85em' }}
        >
          {tok.slice(1, -1)}
        </Box>
      );
    }
    if ((tok.startsWith('*') && tok.endsWith('*')) || (tok.startsWith('_') && tok.endsWith('_'))) {
      return <em key={key}>{tok.slice(1, -1)}</em>;
    }
    return <Fragment key={key}>{tok}</Fragment>;
  });
}

export function RichText({ text }: { text: string }) {
  const lines = text.replace(/\r\n/g, '\n').split('\n');
  const out: ReactNode[] = [];
  let bullets: ReactNode[] = [];

  const flushBullets = (marker: string | number) => {
    if (bullets.length === 0) return;
    out.push(
      <Box component="ul" key={`ul-${marker}`} sx={{ pl: 3, my: 0.5 }}>
        {bullets}
      </Box>,
    );
    bullets = [];
  };

  lines.forEach((line, i) => {
    const bullet = line.match(/^\s*[-*]\s+(.*)$/);
    if (bullet) {
      bullets.push(
        <Typography component="li" variant="body2" key={`li-${i}`}>
          {renderInline(bullet[1], `li-${i}`)}
        </Typography>,
      );
      return;
    }
    flushBullets(i);
    if (line.trim() === '') {
      out.push(<Box key={`sp-${i}`} sx={{ height: 6 }} />);
      return;
    }
    out.push(
      <Typography variant="body2" key={`p-${i}`} sx={{ whiteSpace: 'pre-wrap' }}>
        {renderInline(line, `p-${i}`)}
      </Typography>,
    );
  });
  flushBullets('end');

  return <>{out}</>;
}
