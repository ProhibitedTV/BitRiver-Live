import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { viewerApiMocks } from "../test/test-utils";
import { TipDrawer } from "../components/TipDrawer";
import type { CryptoAddress } from "../lib/viewer-api";

const createTipMock = viewerApiMocks.createTip;

const donationAddresses: CryptoAddress[] = [{ currency: "btc", address: "bc1-test-address" }];
const multiCurrencyDonationAddresses: CryptoAddress[] = [
  { currency: "eth", address: "0xabc123", note: "Main" },
  { currency: "btc", address: "bc1-test-address" }
];

function TipDrawerHarness() {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} ref={triggerRef}>
        Open tip drawer
      </button>
      <TipDrawer
        open={open}
        channelId="chan-123"
        channelTitle="Lo-fi Beats"
        donationAddresses={donationAddresses}
        onClose={() => setOpen(false)}
        onSuccess={() => setOpen(false)}
        returnFocusRef={triggerRef}
      />
    </>
  );
}

describe("TipDrawer", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test("submits fractional tip amounts", async () => {
    createTipMock.mockResolvedValue({} as any);

    const handleClose = jest.fn();
    const handleSuccess = jest.fn();

    const user = userEvent.setup();

    render(
      <TipDrawer
        open
        channelId="chan-123"
        channelTitle="Lo-fi Beats"
        donationAddresses={donationAddresses}
        onClose={handleClose}
        onSuccess={handleSuccess}
      />
    );

    const amountInput = screen.getByLabelText("Amount");
    const referenceInput = screen.getByLabelText("Wallet reference");

    await act(async () => {
      await user.type(amountInput, "0.0005");
      await user.type(referenceInput, "txn-123");
      await user.click(screen.getByRole("button", { name: /send tip/i }));
    });

    await waitFor(() => {
      expect(createTipMock).toHaveBeenCalledTimes(1);
      expect(createTipMock).toHaveBeenCalledWith(
        "chan-123",
        expect.objectContaining({ amount: 0.0005 })
      );
    });
  });

  test("returns focus to trigger for escape, backdrop, close, and cancel", async () => {
    const user = userEvent.setup();
    render(<TipDrawerHarness />);

    const trigger = screen.getByRole("button", { name: /open tip drawer/i });

    await user.click(trigger);
    await screen.findByRole("dialog", { name: /send a tip/i });
    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();

    await user.click(trigger);
    const dialogForBackdrop = await screen.findByRole("dialog", { name: /send a tip/i });
    fireEvent.click(dialogForBackdrop.parentElement as HTMLElement);
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();

    await user.click(trigger);
    const dialogForClose = await screen.findByRole("dialog", { name: /send a tip/i });
    await user.click(within(dialogForClose).getByRole("button", { name: /close tip form/i }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();

    await user.click(trigger);
    const dialogForCancel = await screen.findByRole("dialog", { name: /send a tip/i });
    await user.click(within(dialogForCancel).getByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  test("returns focus to trigger after successful submit", async () => {
    createTipMock.mockResolvedValue({} as any);

    const user = userEvent.setup();
    render(<TipDrawerHarness />);

    const trigger = screen.getByRole("button", { name: /open tip drawer/i });
    await user.click(trigger);

    const dialog = await screen.findByRole("dialog", { name: /send a tip/i });
    await user.type(within(dialog).getByLabelText(/amount/i), "0.5");
    await user.type(within(dialog).getByLabelText(/wallet reference/i), "hash-1");
    await user.click(within(dialog).getByRole("button", { name: /send tip/i }));

    await waitFor(() => expect(screen.queryByRole("dialog", { name: /send a tip/i })).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  test("changing currency keeps typed values and updates the matching wallet address", async () => {
    createTipMock.mockResolvedValue({} as any);

    const user = userEvent.setup();
    render(
      <TipDrawer
        open
        channelId="chan-123"
        channelTitle="Lo-fi Beats"
        donationAddresses={multiCurrencyDonationAddresses}
        onClose={jest.fn()}
        onSuccess={jest.fn()}
      />
    );

    const amountInput = screen.getByLabelText("Amount");
    const referenceInput = screen.getByLabelText("Wallet reference");
    const messageInput = screen.getByLabelText("Message (optional)");
    const currencySelect = screen.getByLabelText("Currency");
    const walletAddressInput = screen.getByLabelText("Wallet address (optional)");

    await user.type(amountInput, "0.75");
    await user.type(referenceInput, "txn-btc-1");
    await user.type(messageInput, "Great stream");
    await user.selectOptions(currencySelect, "BTC");

    await waitFor(() => expect(walletAddressInput).toHaveValue("bc1-test-address"));
    expect(amountInput).toHaveValue(0.75);
    expect(referenceInput).toHaveValue("txn-btc-1");
    expect(messageInput).toHaveValue("Great stream");

    await user.click(screen.getByRole("button", { name: /send tip/i }));

    await waitFor(() => {
      expect(createTipMock).toHaveBeenCalledWith(
        "chan-123",
        expect.objectContaining({
          amount: 0.75,
          currency: "BTC",
          reference: "txn-btc-1",
          walletAddress: "bc1-test-address",
          message: "Great stream"
        })
      );
    });
  });
});
