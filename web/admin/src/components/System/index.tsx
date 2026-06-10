import { IconAChilunshezhisheding } from '@panda-wiki/icons';
import { Button } from '@mui/material';
import { useNavigate } from 'react-router-dom';

const System = () => {
  const navigate = useNavigate();

  return (
    <Button
      size='small'
      variant='outlined'
      startIcon={<IconAChilunshezhisheding />}
      onClick={() => navigate('/setting?tab=model-config')}
    >
      系统配置
    </Button>
  );
};

export default System;
