import * as React from "react";
import { cn } from "../utils";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "outline" | "ghost" | "link";
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center rounded-md text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 disabled:pointer-events-none disabled:opacity-50 active:scale-[0.98]",
          variant === "default" &&
            "bg-violet-600 text-white hover:bg-violet-700 shadow-sm",
          variant === "outline" &&
            "border border-zinc-800 bg-transparent text-zinc-300 hover:bg-zinc-900 hover:text-white",
          variant === "ghost" &&
            "text-zinc-400 hover:bg-zinc-900 hover:text-white",
          variant === "link" &&
            "text-violet-400 underline-offset-4 hover:underline",
          "px-4 py-2",
          className,
        )}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";
