"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent, MouseEvent as ReactMouseEvent } from "react";
import type { CryptoAddress } from "../lib/viewer-api";
import { createTip } from "../lib/viewer-api";

export type TipDrawerProps = {
  open: boolean;
  channelId: string;
  channelTitle: string;
  donationAddresses: CryptoAddress[];
  onClose: () => void;
  onSuccess: (statusMessage: string) => void;
};

const CUSTOM_CURRENCY_OPTION = "__custom__";

export function TipDrawer({
  open,
  channelId,
  channelTitle,
  donationAddresses,
  onClose,
  onSuccess
}: TipDrawerProps) {
  const currencyOptions = useMemo(() => {
    const seen = new Set<string>();
    return donationAddresses
      .map((address) => address.currency?.toUpperCase() ?? "")
      .filter((currency) => {
        const normalized = currency.trim();
        if (!normalized || seen.has(normalized)) {
          return false;
        }
        seen.add(normalized);
        return true;
      });
  }, [donationAddresses]);

  const [currencySelection, setCurrencySelection] = useState<string>(
    currencyOptions[0] ?? (currencyOptions.length === 0 ? CUSTOM_CURRENCY_OPTION : "")
  );
  const [customCurrency, setCustomCurrency] = useState("");
  const [amount, setAmount] = useState("");
  const [reference, setReference] = useState("");
  const [message, setMessage] = useState("");
  const [walletAddress, setWalletAddress] = useState("");
  const [error, setError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);

  const drawerRef = useRef<HTMLElement | null>(null);
  const firstFieldRef = useRef<HTMLInputElement | null>(null);

  const currentCurrency =
    currencySelection === CUSTOM_CURRENCY_OPTION
      ? customCurrency.trim()
      : currencySelection.trim();

  const matchingAddresses = useMemo(
    () =>
      donationAddresses.filter((address) =>
        address.currency
          ? address.currency.toUpperCase() === currentCurrency.toUpperCase()
          : false
      ),
    [donationAddresses, currentCurrency]
  );

  useEffect(() => {
    if (!open) {
      return;
    }
    setError(undefined);
    setSubmitting(false);
    setAmount("");
    setReference("");
    setMessage("");
    if (currencyOptions.length === 0) {
      setCurrencySelection(CUSTOM_CURRENCY_OPTION);
    } else {
      setCurrencySelection(currencyOptions[0] ?? "");
    }
    setCustomCurrency("");
    setWalletAddress(matchingAddresses[0]?.address ?? "");
  }, [open, currencyOptions, matchingAddresses]);

  useEffect(() => {
    if (!open) {
      return;
    }
    firstFieldRef.current?.focus({ preventScroll: true });
  }, [open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  useEffect(() => {
    if (!open) {
      return;
    }
    setWalletAddress(matchingAddresses[0]?.address ?? "");
  }, [matchingAddresses, open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const drawerElement = drawerRef.current;
    if (!drawerElement) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Tab") {
        return;
      }
      const focusableSelectors =
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])';
      const focusable = Array.from(
        drawerElement.querySelectorAll<HTMLElement>(focusableSelectors)
      ).filter((element) => {
        if (element.hasAttribute("disabled")) {
          return false;
        }
        if (element.getAttribute("aria-hidden") === "true") {
          return false;
        }
        if (element.tabIndex === -1) {
          return false;
        }
        if (element === document.activeElement) {
          return true;
        }
        return element.offsetParent !== null || element.getClientRects().length > 0;
      });
      if (focusable.length === 0) {
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (!active || !drawerElement.contains(active)) {
        event.preventDefault();
        first.focus();
        return;
      }
      if (event.shiftKey) {
        if (active === first) {
          event.preventDefault();
          last.focus();
        }
        return;
      }
      if (active === last) {
        event.preventDefault();
        first.focus();
      }
    };
    drawerElement.addEventListener("keydown", handleKeyDown);
    return () => drawerElement.removeEventListener("keydown", handleKeyDown);
  }, [open]);

  const handleBackdropClick = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) {
      onClose();
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const numericAmount = Number.parseFloat(amount);
    if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
      setError("Enter a valid amount greater than zero.");
      return;
    }
    if (!currentCurrency) {
      setError("Select or enter a currency for your tip.");
      return;
    }
    const trimmedReference = reference.trim();
    if (trimmedReference.length === 0) {
      setError("Provide the wallet or transaction reference so the creator can reconcile it.");
      return;
    }
    setSubmitting(true);
    setError(undefined);
    try {
      await createTip(channelId, {
        amount: numericAmount,
        currency: currentCurrency,
        provider: "viewer",
        reference: trimmedReference,
        walletAddress: walletAddress.trim() ? walletAddress.trim() : undefined,
        message: message.trim() ? message.trim() : undefined
      });
      onSuccess(`Thanks for supporting ${channelTitle}!`);
    } catch (err) {
      const fallback = "Unable to send the tip right now. Try again in a moment.";
      setError(err instanceof Error ? err.message || fallback : fallback);
    } finally {
      setSubmitting(false);
    }
  };

  if (!open) {
    return null;
  }

  return (
    <div className="tip-drawer__backdrop" onClick={handleBackdropClick}>
      <section
        className="tip-drawer surface stack"
        role="dialog"
        aria-modal="true"
        aria-labelledby="tip-drawer-title"
        ref={drawerRef}
      >
        <header className="tip-drawer__header">
          <div className="stack" style={{ gap: "0.35rem" }}>
            <p className="muted">Support {channelTitle}</p>
            <h2 id="tip-drawer-title">Send a tip</h2>
          </div>
          <button type="button" className="tip-drawer__close" onClick={onClose} aria-label="Close tip form">
            ×
          </button>
        </header>
        <form className="tip-drawer__form stack" onSubmit={handleSubmit}>
          <div className="tip-drawer__field-group">
            <label htmlFor="tip-amount">Amount</label>
            <input
              id="tip-amount"
              name="amount"
              type="number"
              min="0"
              step="any"
              inputMode="decimal"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              required
              ref={firstFieldRef}
            />
          </div>
          <div className="tip-drawer__field-group">
            <label htmlFor="tip-currency">Currency</label>
            <select
              id="tip-currency"
              value={currencySelection}
              onChange={(event) => setCurrencySelection(event.target.value)}
            >
              <option value="">Select a currency</option>
              {currencyOptions.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
              <option value={CUSTOM_CURRENCY_OPTION}>Other</option>
            </select>
            {currencySelection === CUSTOM_CURRENCY_OPTION && (
              <input
                id="tip-currency-custom"
                name="customCurrency"
                placeholder="e.g. USD"
                value={customCurrency}
                onChange={(event) => setCustomCurrency(event.target.value)}
                aria-label="Custom currency"
                required
              />
            )}
          </div>
          <div className="tip-drawer__field-group">
            <label htmlFor="tip-reference">Wallet reference</label>
            <input
              id="tip-reference"
              name="reference"
              placeholder="Transaction hash or wallet handle"
              value={reference}
              onChange={(event) => setReference(event.target.value)}
              required
            />
          </div>
          <div className="tip-drawer__field-group">
            <label htmlFor="tip-wallet-address">Wallet address (optional)</label>
            <input
              id="tip-wallet-address"
              name="walletAddress"
              placeholder="Paste the address you tipped"
              value={walletAddress}
              onChange={(event) => setWalletAddress(event.target.value)}
            />
            {matchingAddresses.length > 0 && (
              <div className="tip-drawer__address-hint">
                <p className="muted">Creator addresses for {currentCurrency || "this currency"}:</p>
                <ul>
                  {matchingAddresses.map((address, index) => {
                    const key = `${address.currency}-${address.address}-${index}`;
                    return (
                      <li key={key}>
                        <button
                          type="button"
                          onClick={() => setWalletAddress(address.address)}
                          className="tip-drawer__address-option"
                        >
                          {address.address}
                          {address.note ? ` · ${address.note}` : ""}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            )}
          </div>
          <div className="tip-drawer__field-group">
            <label htmlFor="tip-message">Message (optional)</label>
            <textarea
              id="tip-message"
              name="message"
              rows={3}
              value={message}
              onChange={(event) => setMessage(event.target.value)}
              placeholder="Add a note to the creator"
            />
          </div>
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <div className="tip-drawer__actions">
            <button type="submit" className="primary-button" disabled={submitting}>
              {submitting ? "Sending…" : "Send tip"}
            </button>
            <button type="button" className="secondary-button" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
