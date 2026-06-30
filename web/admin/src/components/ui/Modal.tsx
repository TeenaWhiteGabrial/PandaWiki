import {
  Modal as AntdModal,
  type ModalFuncProps,
  type ModalProps as AntdModalProps,
} from 'antd';
import type { ButtonProps } from '@mui/material';
import type { SxProps, Theme } from '@mui/material/styles';
import type { CSSProperties, ReactNode } from 'react';

type MuiModalButtonProps = Omit<ButtonProps, 'loading'> & {
  loading?: boolean | null;
};

type FooterRender = (props: {
  OkBtn: ReactNode;
  CancelBtn: ReactNode;
}) => ReactNode;

export interface ModalProps
  extends Omit<
    AntdModalProps,
    | 'title'
    | 'open'
    | 'footer'
    | 'onOk'
    | 'onCancel'
    | 'styles'
    | 'okButtonProps'
    | 'cancelButtonProps'
    | 'closable'
    | 'keyboard'
    | 'width'
  > {
  open?: boolean;
  title?: ReactNode;
  width?: number | string;
  children?: ReactNode;
  footer?: null | ReactNode | FooterRender;
  okText?: ReactNode;
  cancelText?: ReactNode;
  cancelButtonProps?: MuiModalButtonProps;
  okButtonProps?: MuiModalButtonProps;
  showCancel?: boolean;
  maskClosable?: boolean;
  keyboard?: boolean;
  closable?: boolean;
  disableEscapeKeyDown?: boolean;
  transitionDuration?: number;
  sx?: SxProps<Theme>;
  onOk?(): void;
  onClose?(e?: unknown, r?: unknown): void;
  onCancel?(): void;
}

export interface ConfirmDialogProps
  extends Omit<ModalProps, 'children' | 'footer' | 'open'> {
  content?: ReactNode;
  icon?: ReactNode;
}

const toStyleObject = (sx?: SxProps<Theme>): CSSProperties | undefined => {
  if (!sx || Array.isArray(sx) || typeof sx === 'function') return undefined;
  return sx as CSSProperties;
};

const mapButtonProps = (
  props?: MuiModalButtonProps,
): AntdModalProps['okButtonProps'] => {
  if (!props) return undefined;
  const { sx, loading, variant, color, ...rest } = props;
  return {
    ...rest,
    loading: loading ?? undefined,
    type:
      variant === 'contained'
        ? 'primary'
        : variant === 'text'
          ? 'text'
          : variant === 'outlined'
            ? 'default'
            : undefined,
    danger: color === 'error',
    style: {
      ...(rest.style as CSSProperties | undefined),
      ...(toStyleObject(sx) || {}),
    },
  };
};

const renderFooter = (
  footer: ModalProps['footer'],
  showCancel: boolean,
  cancelText: ReactNode,
  okText: ReactNode,
  cancelButtonProps?: MuiModalButtonProps,
  okButtonProps?: MuiModalButtonProps,
  onCancel?: () => void,
  onOk?: () => void,
) => {
  const cancelNode = showCancel ? (
    <button
      key='cancel'
      type='button'
      onClick={onCancel}
      style={toStyleObject(cancelButtonProps?.sx)}
    >
      {cancelText}
    </button>
  ) : null;
  const okNode = (
    <button
      key='ok'
      type='button'
      onClick={onOk}
      disabled={okButtonProps?.disabled}
      style={toStyleObject(okButtonProps?.sx)}
    >
      {okText}
    </button>
  );

  if (footer === null) return null;
  if (typeof footer === 'function') {
    return footer({
      OkBtn: okNode,
      CancelBtn: cancelNode,
    });
  }
  return footer;
};

const Modal = ({
  open,
  title,
  width,
  children,
  footer,
  okText = '确认',
  cancelText = '取消',
  cancelButtonProps,
  okButtonProps,
  showCancel = true,
  maskClosable = true,
  keyboard = true,
  closable = true,
  disableEscapeKeyDown,
  onOk,
  onClose,
  onCancel,
  sx,
  style,
  ...rest
}: ModalProps) => {
  const footerNode = renderFooter(
    footer,
    showCancel,
    cancelText,
    okText,
    cancelButtonProps,
    okButtonProps,
    onCancel,
    onOk,
  );

  return (
    <AntdModal
      open={open}
      title={title}
      width={width}
      onOk={onOk}
      onCancel={event => {
        onCancel?.();
        onClose?.(event, undefined);
      }}
      okText={okText}
      cancelText={cancelText}
      okButtonProps={mapButtonProps(okButtonProps)}
      cancelButtonProps={mapButtonProps(cancelButtonProps)}
      maskClosable={maskClosable}
      keyboard={disableEscapeKeyDown ? false : keyboard}
      closable={closable}
      footer={footerNode}
      style={{
        ...style,
        ...(toStyleObject(sx) || {}),
      }}
      {...rest}
    >
      {children}
    </AntdModal>
  );
};

Modal.confirm = (config: ConfirmDialogProps) => {
  const {
    title,
    content,
    icon,
    okText = '确认',
    cancelText = '取消',
    onOk,
    onCancel,
    okButtonProps,
    cancelButtonProps,
    width,
    ...rest
  } = config;

  return AntdModal.confirm({
    title,
    content,
    icon,
    okText,
    cancelText,
    onOk,
    onCancel,
    width,
    okButtonProps: mapButtonProps(okButtonProps),
    cancelButtonProps: mapButtonProps(cancelButtonProps),
    ...rest,
  } as ModalFuncProps);
};

export default Modal as typeof Modal & {
  confirm: (config: ConfirmDialogProps) => ReturnType<typeof AntdModal.confirm>;
};
