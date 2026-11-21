import type { InputHTMLAttributes } from 'react';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export function Input({ label, error, id, className = '', ...rest }: InputProps) {
  return (
    <label className="block w-full">
      {label && (
        <span className="block text-xs font-semibold mb-1 text-slate-700">
          {label}
        </span>
      )}
      <input
        id={id}
        className={`w-full h-9 px-3 py-2 rounded-lg border bg-white text-sm transition-colors focus:outline-none focus:ring-2 focus:ring-brand/30 ${
          error ? 'border-rose-500 text-rose-900 focus:ring-rose-200' : 'border-slate-200 text-slate-900 focus:border-brand'
        } ${className}`}
        {...rest}
      />
      {error && <span className="block text-xs text-rose-600 mt-1">{error}</span>}
    </label>
  );
}
