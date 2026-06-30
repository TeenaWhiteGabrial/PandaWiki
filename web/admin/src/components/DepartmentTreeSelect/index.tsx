import type { Department } from '@/request/Enterprise';
import { normalizeSelectedDepartmentIDs } from '@/utils/enterprise';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import { SimpleTreeView } from '@mui/x-tree-view/SimpleTreeView';
import { TreeItem } from '@mui/x-tree-view/TreeItem';
import {
  Box,
  Checkbox,
  FormHelperText,
  InputAdornment,
  TextField,
  Typography,
  styled,
} from '@mui/material';
import { forwardRef, useCallback, useEffect, useMemo, useState } from 'react';

export interface DepartmentTreeSelectProps {
  departments: Department[];
  flatDepartments: Department[];
  value: string[];
  onChange: (value: string[]) => void;
  multiple?: boolean;
  label?: string;
  placeholder?: string;
  disabled?: boolean;
  excludeIds?: string[];
  error?: boolean;
  helperText?: string;
  size?: 'small' | 'medium';
}

const StyledPopper = styled('div')`
  position: absolute;
  z-index: 1300;
  background-color: ${({ theme }) =>
    theme.palette.mode === 'dark' ? '#1e1e1e' : '#fff'};
  border: 1px solid
    ${({ theme }) => (theme.palette.mode === 'dark' ? '#333' : '#ccc')};
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  max-height: 480px;
  overflow-y: auto;
  padding: 4px 0;
`;

const formatSelectedNames = (items: Department[], multiple: boolean) => {
  if (!items.length) return '';
  if (!multiple) return items[0]?.name || '';
  return items.map(item => item.name).join('、');
};

/** 单个树节点，支持选中/取消选中逻辑 */
const DepartmentTreeItem = ({
  dept,
  selectedIds,
  excludeIds,
  onSelect,
}: {
  dept: Department;
  selectedIds: string[];
  excludeIds: Set<string>;
  onSelect: (id: string) => void;
}) => {
  const isSelected = selectedIds.includes(dept.id);
  const isExcluded = excludeIds.has(dept.id);

  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (!isExcluded) onSelect(dept.id);
    },
    [isExcluded, onSelect, dept.id],
  );

  return (
    <TreeItem
      itemId={dept.id}
      label={
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 0.5,
            py: 0.25,
            opacity: isExcluded ? 0.5 : 1,
            cursor: isExcluded ? 'not-allowed' : 'pointer',
          }}
          onClick={handleClick}
        >
          <Checkbox
            size='small'
            checked={isSelected}
            disabled={isExcluded}
            onClick={handleClick}
            sx={{ p: 0.25 }}
          />
          <Typography variant='body2'>{dept.name}</Typography>
        </Box>
      }
    >
      {dept.children?.length
        ? dept.children.map(child => (
            <DepartmentTreeItem
              key={child.id}
              dept={child}
              selectedIds={selectedIds}
              excludeIds={excludeIds}
              onSelect={onSelect}
            />
          ))
        : null}
    </TreeItem>
  );
};

const DepartmentTreeSelect = forwardRef<
  HTMLDivElement,
  DepartmentTreeSelectProps
>(
  (
    {
      departments,
      flatDepartments,
      value,
      onChange,
      multiple = true,
      placeholder = '请选择部门',
      disabled = false,
      excludeIds = [],
      error,
      helperText,
      size = 'small',
    },
    ref,
  ) => {
    const [open, setOpen] = useState(false);
    const [expandedItems, setExpandedItems] = useState<string[]>([]);
    const excludeIdSet = useMemo(() => new Set(excludeIds), [excludeIds]);

    const departmentMap = useMemo(() => {
      const map = new Map<string, Department>();
      flatDepartments.forEach(d => map.set(d.id, d));
      return map;
    }, [flatDepartments]);

    const selectedItems = useMemo(
      () =>
        value
          .map(id => departmentMap.get(id))
          .filter((d): d is Department => !!d),
      [value, departmentMap],
    );
    const selectedNames = useMemo(
      () => formatSelectedNames(selectedItems, multiple),
      [selectedItems, multiple],
    );

    const handleSelect = useCallback(
      (nodeId: string) => {
        if (disabled) return;
        if (excludeIdSet.has(nodeId)) return;

        let newValue: string[];
        if (!multiple) {
          newValue = value.includes(nodeId) ? [] : [nodeId];
        } else if (value.includes(nodeId)) {
          newValue = value.filter(id => id !== nodeId);
        } else {
          newValue = normalizeSelectedDepartmentIDs(
            [...value, nodeId],
            flatDepartments,
            nodeId,
          );
        }

        onChange(newValue);
        if (!multiple) {
          setOpen(false);
        }
      },
      [value, onChange, multiple, flatDepartments, disabled, excludeIdSet],
    );

    const ancestorExpandedIds = useMemo(() => {
      const ids = new Set<string>();
      const collectAncestors = (nodeId: string) => {
        const dept = departmentMap.get(nodeId);
        if (dept?.parent_id) {
          ids.add(dept.parent_id);
          collectAncestors(dept.parent_id);
        }
      };
      value.forEach(id => collectAncestors(id));
      return [...ids];
    }, [value, departmentMap]);

    useEffect(() => {
      setExpandedItems(prev => [...new Set([...prev, ...ancestorExpandedIds])]);
    }, [ancestorExpandedIds]);

    const handleOpen = useCallback(
      (event?: React.MouseEvent | React.FocusEvent) => {
        event?.preventDefault?.();
        event?.stopPropagation?.();
        if (!disabled) {
          setOpen(true);
        }
      },
      [disabled],
    );

    const handlePopperMouseDown = useCallback((event: React.MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();
    }, []);

    const handlePopperClick = useCallback((event: React.MouseEvent) => {
      event.stopPropagation();
    }, []);

    // 点击外部关闭
    useEffect(() => {
      if (!open) return;
      const handleClickOutside = (e: MouseEvent) => {
        const target = e.target as HTMLElement;
        if (!target.closest('[data-department-tree-select]')) {
          setOpen(false);
        }
      };
      document.addEventListener('mousedown', handleClickOutside);
      return () =>
        document.removeEventListener('mousedown', handleClickOutside);
    }, [open]);

    return (
      <Box
        ref={ref}
        data-department-tree-select
        sx={{ position: 'relative', width: '100%' }}
      >
        <TextField
          fullWidth
          value={selectedNames}
          placeholder={placeholder}
          error={error}
          size={size}
          disabled={disabled}
          onClick={handleOpen}
          onMouseDown={handleOpen}
          InputProps={{
            readOnly: true,
            endAdornment: (
              <InputAdornment position='end'>
                <KeyboardArrowDownIcon
                  sx={{
                    fontSize: 20,
                    color: 'text.secondary',
                    transform: open ? 'rotate(180deg)' : 'rotate(0deg)',
                    transition: 'transform 0.2s ease',
                  }}
                />
              </InputAdornment>
            ),
          }}
          inputProps={{
            readOnly: true,
            style: { cursor: disabled ? 'not-allowed' : 'pointer' },
          }}
          sx={{
            '& .MuiFormLabel-root': {
              display: 'none',
            },
            '& .MuiOutlinedInput-root': {
              cursor: disabled ? 'not-allowed' : 'pointer',
            },
            '& .MuiInputBase-input::placeholder': {
              color: 'text.secondary',
              opacity: selectedNames ? 0 : 1,
            },
          }}
        />
        {helperText ? (
          <FormHelperText error={error} sx={{ mx: 1.75 }}>
            {helperText}
          </FormHelperText>
        ) : null}

        {open && (
          <StyledPopper
            sx={{
              top: '100%',
              left: 0,
              right: 0,
              mt: 0.5,
            }}
            onMouseDown={handlePopperMouseDown}
            onClick={handlePopperClick}
          >
            <SimpleTreeView
              expandedItems={expandedItems}
              onExpandedItemsChange={(_, itemIds) =>
                setExpandedItems(itemIds as string[])
              }
              sx={{ py: 0.5 }}
            >
              {departments.map(dept => (
                <DepartmentTreeItem
                  key={dept.id}
                  dept={dept}
                  selectedIds={value}
                  excludeIds={excludeIdSet}
                  onSelect={handleSelect}
                />
              ))}
            </SimpleTreeView>
          </StyledPopper>
        )}
      </Box>
    );
  },
);

DepartmentTreeSelect.displayName = 'DepartmentTreeSelect';

export default DepartmentTreeSelect;
