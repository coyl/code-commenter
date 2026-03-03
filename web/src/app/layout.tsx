import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Code Commenter Live Agent",
  description: "Describe a coding task with text or voice, get CSS + code with typing effect and voiceover.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen antialiased">{children}</body>
    </html>
  );
}
