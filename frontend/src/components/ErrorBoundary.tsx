import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Box, Button, Typography } from '@mui/material';

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

// Top-level render-error guard: without this, any uncaught error while
// rendering unmounts the whole React tree (React 18 default), leaving a
// blank white screen with nothing shown to the user.
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled render error', error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      return (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', px: 3, textAlign: 'center' }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            Что-то пошло не так
          </Typography>
          <Typography color="text.secondary" sx={{ mb: 3 }}>
            Попробуйте перезагрузить страницу
          </Typography>
          <Button variant="contained" onClick={() => window.location.reload()}>
            Перезагрузить
          </Button>
        </Box>
      );
    }

    return this.props.children;
  }
}
