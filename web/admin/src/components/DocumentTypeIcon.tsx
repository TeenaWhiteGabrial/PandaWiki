import ArticleOutlinedIcon from '@mui/icons-material/ArticleOutlined';
import CodeOutlinedIcon from '@mui/icons-material/CodeOutlined';
import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import LanguageOutlinedIcon from '@mui/icons-material/LanguageOutlined';
import PictureAsPdfOutlinedIcon from '@mui/icons-material/PictureAsPdfOutlined';
import SlideshowOutlinedIcon from '@mui/icons-material/SlideshowOutlined';
import TableChartOutlinedIcon from '@mui/icons-material/TableChartOutlined';

interface DocumentTypeIconProps {
  contentType?: string;
  fileName?: string;
  sx?: Record<string, unknown>;
}

const DocumentTypeIcon = ({
  contentType,
  fileName,
  sx,
}: DocumentTypeIconProps) => {
  const type = `${contentType || ''} ${fileName || ''}`.toLowerCase();
  const iconSx = { fontSize: 26, ...sx };

  if (type.includes('pdf')) {
    return <PictureAsPdfOutlinedIcon sx={{ ...iconSx, color: 'error.main' }} />;
  }
  if (type.includes('html') || type.includes('.htm')) {
    return <LanguageOutlinedIcon sx={{ ...iconSx, color: 'warning.main' }} />;
  }
  if (
    type.includes('excel') ||
    type.includes('sheet') ||
    type.includes('spreadsheet') ||
    type.includes('csv') ||
    /\.(xls|xlsx|csv)(\s|$)/.test(type)
  ) {
    return <TableChartOutlinedIcon sx={{ ...iconSx, color: 'success.main' }} />;
  }
  if (
    type.includes('powerpoint') ||
    type.includes('presentation') ||
    /\.(ppt|pptx)(\s|$)/.test(type)
  ) {
    return <SlideshowOutlinedIcon sx={{ ...iconSx, color: 'warning.dark' }} />;
  }
  if (
    type.includes('markdown') ||
    type.includes('.md') ||
    type.includes('text/plain')
  ) {
    return <CodeOutlinedIcon sx={{ ...iconSx, color: 'secondary.main' }} />;
  }
  if (type.includes('word') || type.includes('document')) {
    return <ArticleOutlinedIcon sx={{ ...iconSx, color: 'primary.main' }} />;
  }
  return <DescriptionOutlinedIcon sx={{ ...iconSx, color: 'primary.main' }} />;
};

export default DocumentTypeIcon;
