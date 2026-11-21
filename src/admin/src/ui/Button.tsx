import type { ButtonHTMLAttributes, ReactNode } from 'react';

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
  children: ReactNode;
}

const variantClasses: Record<Variant, string> = {
  primary: 'bg-brand hover:bg-brand-600 text-white border-brand shadow-xs',
  secondary: 'bg-white hover:bg-slate-50 text-slate-700 border-slate-200 shadow-xs',
  danger: 'bg-rose-600 hover:bg-rose-700 text-white border-rose-600 shadow-xs',
  ghost: 'bg-transparent hover:bg-slate-100 text-slate-700 border-transparent',
};

export function Button({ variant = 'primary', loading, disabled, children, className = '', ...rest }: ButtonProps) {
  const isInactive = disabled || loading;
  return (
    <button
      disabled={isInactive}
      className={`inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-lg text-sm font-semibold border transition-all duration-150 ${variantClasses[variant]} ${isInactive ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'} ${className}`}
      {...rest}
    >
      {loading ? '…' : children}
    </button>
  );
}
