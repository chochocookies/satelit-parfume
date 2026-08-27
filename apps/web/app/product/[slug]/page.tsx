import { ProductDetailClient } from "@/components/product-detail-client";

// Next.js 16: params is a Promise, must be awaited before use.
export default async function ProductPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  return <ProductDetailClient slug={slug} />;
}
