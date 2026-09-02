import { useEffect, useState } from 'react';
import { Box, Dialog, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';

interface ImageLightboxProps {
  src: string | null;
  alt?: string;
  onClose: () => void;
}

/** Fullscreen image viewer. Click the image to zoom in/out, click the backdrop
 * or the close button to dismiss. */
export function ImageLightbox({ src, alt, onClose }: ImageLightboxProps) {
  const [zoomed, setZoomed] = useState(false);

  useEffect(() => {
    if (!src) setZoomed(false);
  }, [src]);

  return (
    <Dialog
      open={Boolean(src)}
      onClose={onClose}
      maxWidth="lg"
      fullWidth
      PaperProps={{ sx: { bgcolor: 'transparent', boxShadow: 'none', m: 1 } }}
    >
      <Box
        onClick={onClose}
        sx={{
          position: 'relative',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '60vh',
          overflow: 'auto',
        }}
      >
        <IconButton
          onClick={onClose}
          aria-label="Закрыть"
          sx={{ position: 'absolute', top: 8, right: 8, color: '#fff', bgcolor: 'rgba(0,0,0,0.5)' }}
        >
          <CloseIcon />
        </IconButton>
        {src && (
          <Box
            component="img"
            src={src}
            alt={alt ?? ''}
            decoding="async"
            onClick={(e) => {
              e.stopPropagation();
              setZoomed((z) => !z);
            }}
            sx={{
              maxWidth: '100%',
              maxHeight: '85vh',
              objectFit: 'contain',
              borderRadius: 2,
              cursor: zoomed ? 'zoom-out' : 'zoom-in',
              transform: zoomed ? 'scale(1.8)' : 'scale(1)',
              transition: 'transform 0.25s ease',
            }}
          />
        )}
      </Box>
    </Dialog>
  );
}
