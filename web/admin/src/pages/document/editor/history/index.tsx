import { Box } from '@mui/material';

const History = () => {
  return (
    <Box
      sx={{
        p: 4,
        color: 'text.secondary',
        fontSize: 14,
        lineHeight: '24px',
      }}
    >
      当前环境未启用历史版本能力。
    </Box>
  );
};

export default History;
