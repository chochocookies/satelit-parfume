"use client";

import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient } from "@/lib/api-client";
import { useCartStore } from "@/stores/cart-store";

// Central place every cart-touching component reads/writes through —
// keeps the "which header, which query key" details in one spot instead
// of repeated in the header cart button, the cart panel, and the product
// page's Add to Cart control.
export function useCart() {
  const cartToken = useCartStore((s) => s.cartToken);
  const hasHydrated = useCartStore((s) => s.hasHydrated);
  const setCartToken = useCartStore((s) => s.setCartToken);
  const queryClient = useQueryClient();

  const cartQuery = useQuery({
    queryKey: ["cart", cartToken],
    queryFn: () => apiClient.getCart(cartToken),
    // Wait for localStorage to load first — fetching once with
    // cartToken=null and then immediately refetching with the real
    // saved token would create a brand new guest cart every page load.
    enabled: hasHydrated,
  });

  // A first-time guest gets a session_token back from the server; persist
  // it so the *next* request (including this same query, re-keyed) uses
  // it instead of creating yet another cart. TanStack Query v5 dropped
  // useQuery's onSuccess callback, so syncing query data into an external
  // store (Zustand + localStorage) is what useEffect is actually for here.
  useEffect(() => {
    const token = cartQuery.data?.session_token;
    if (token && token !== cartToken) {
      setCartToken(token);
    }
  }, [cartQuery.data?.session_token, cartToken, setCartToken]);

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["cart"] });
  }

  const addItem = useMutation({
    mutationFn: (item: { product_variant_id: string; branch_id: string; quantity: number }) =>
      apiClient.addCartItem(cartToken, item),
    onSuccess: (cart) => {
      if (cart.session_token) setCartToken(cart.session_token);
      invalidate();
    },
  });

  const updateItem = useMutation({
    mutationFn: ({ itemId, quantity }: { itemId: string; quantity: number }) =>
      apiClient.updateCartItem(cartToken, itemId, quantity),
    onSuccess: invalidate,
  });

  const removeItem = useMutation({
    mutationFn: (itemId: string) => apiClient.removeCartItem(cartToken, itemId),
    onSuccess: invalidate,
  });

  return {
    cart: cartQuery.data,
    isLoading: !hasHydrated || cartQuery.isLoading,
    isError: cartQuery.isError,
    addItem,
    updateItem,
    removeItem,
  };
}
