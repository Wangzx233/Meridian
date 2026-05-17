export function MeridianIcon(props: { size?: number; className?: string }) {
  const size = props.size ?? 24;

  return (
    <svg
      className={props.className}
      width={size}
      height={size}
      viewBox="0 0 64 64"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      focusable="false"
    >
      <rect width="64" height="64" rx="14" fill="#DCE9DF" />
      <path d="M32 18v28" stroke="#F6F7F2" strokeWidth="10" strokeLinecap="round" />
      <path d="M32 18c-15 7-15 21 0 28" stroke="#28513D" strokeWidth="5" strokeLinecap="round" />
      <path d="M32 18c15 7 15 21 0 28" stroke="#28513D" strokeWidth="5" strokeLinecap="round" />
      <path d="M32 18v28" stroke="#28513D" strokeWidth="5" strokeLinecap="round" />
      <circle cx="32" cy="18" r="4" fill="#AD6B20" />
      <circle cx="32" cy="46" r="4" fill="#AD6B20" />
    </svg>
  );
}
