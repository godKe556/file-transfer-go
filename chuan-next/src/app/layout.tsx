import type { Metadata } from "next";
import "./globals.css";
import { ToastProvider } from "@/components/ui/toast-simple";

export const metadata: Metadata = {
  title: "文件快传",
  description: "安全、快速、简单的文件传输服务",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <head>
        <script defer src="https://track.icats.club/script.js" data-website-id="0223f119-aaaa-4bd4-b8f7-33aeeb2bb927"></script>
      </head>
      <body className="antialiased">
        <ToastProvider>
          {children}
        </ToastProvider>
      </body>
    </html>
  );
}
