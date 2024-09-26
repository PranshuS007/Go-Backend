package db

import (
	"context"

	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransferTx(t *testing.T) {
	store := NewStore(testDB)

	account1 := createRandomAccount(t)
	account2 := createRandomAccount(t)
	fmt.Println(">>>> Initial Balances:", account1.Balance, account2.Balance)

	// Run it with goroutines
	n := 5 // Concurrent transactions
	amount := int64(10)

	errs := make(chan error)
	results := make(chan TransferTxResult)

	for i := 0; i < n; i++ {
		go func(i int) {
			fmt.Printf("Starting transaction %d\n", i+1)
			result, err := store.TransferTx(context.Background(), TransferTxParams{
				FromAccountID: account1.ID,
				ToAccountID:   account2.ID,
				Amount:        amount,
			})
			if err != nil {
				fmt.Printf("Transaction %d failed: %v\n", i+1, err)
			} else {
				fmt.Printf("Transaction %d succeeded: %+v\n", i+1, result)
			}
			errs <- err
			results <- result
		}(i)
	}

	// Check results
	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)

		result := <-results
		require.NotEmpty(t, result)

		transfer := result.Transfer
		require.NotEmpty(t, transfer)
		require.Equal(t, account1.ID, transfer.FromAccountID)
		require.Equal(t, account2.ID, transfer.ToAccountID)
		require.Equal(t, amount, transfer.Amount)
		require.NotZero(t, transfer.ID)
		require.NotZero(t, transfer.CreatedAt)

		fmt.Printf("Verifying Transfer ID: %d\n", transfer.ID)
		_, err = store.GetTransfer(context.Background(), transfer.ID)
		if err != nil {
			fmt.Printf("Failed to get Transfer ID %d: %v\n", transfer.ID, err)
		}
		require.NoError(t, err)

		fromEntry := result.FromEntry
		require.NotEmpty(t, fromEntry)
		require.Equal(t, account1.ID, fromEntry.AccountID)
		require.Equal(t, -amount, fromEntry.Amount)
		require.NotZero(t, fromEntry.ID)
		require.NotZero(t, fromEntry.CreatedAt)

		fmt.Printf("Verifying FromEntry ID: %d\n", fromEntry.ID)
		_, err = store.GetEntry(context.Background(), fromEntry.ID)
		if err != nil {
			fmt.Printf("Failed to get FromEntry ID %d: %v\n", fromEntry.ID, err)
		}
		require.NoError(t, err)

		toEntry := result.ToEntry
		require.NotEmpty(t, toEntry)
		require.Equal(t, account2.ID, toEntry.AccountID)
		require.Equal(t, amount, toEntry.Amount)
		require.NotZero(t, toEntry.ID)
		require.NotZero(t, toEntry.CreatedAt)

		fmt.Printf("Verifying ToEntry ID: %d\n", toEntry.ID)
		_, err = store.GetEntry(context.Background(), toEntry.ID)
		if err != nil {
			fmt.Printf("Failed to get ToEntry ID %d: %v\n", toEntry.ID, err)
		}
		require.NoError(t, err)

		fromAccount := result.FromAccount
		require.NotEmpty(t, fromAccount)
		require.Equal(t, account1.ID, fromAccount.ID)

		toAccount := result.ToAccount
		require.NotEmpty(t, toAccount)
		require.Equal(t, account2.ID, toAccount.ID)

		fmt.Println(">> Updated Balances:", fromAccount.Balance, toAccount.Balance)
		diff1 := account1.Balance - fromAccount.Balance
		diff2 := toAccount.Balance - account2.Balance

		require.Equal(t, diff1, diff2)
		require.True(t, diff1 > 0)
		require.True(t, diff1%amount == 0)
	}

	updatedAccount1, err := testQueries.GetAccount(context.Background(), account1.ID)
	require.NoError(t, err)

	updatedAccount2, err := testQueries.GetAccount(context.Background(), account2.ID)
	require.NoError(t, err)

	fmt.Println("<<<<< Final Balances:", updatedAccount1.Balance, updatedAccount2.Balance)
	require.Equal(t, account1.Balance-int64(n)*amount, updatedAccount1.Balance)
	require.Equal(t, account2.Balance+int64(n)*amount, updatedAccount2.Balance)
}

func TestTransferTxDeadlock(t *testing.T) {
	store := NewStore(testDB)

	account1 := createRandomAccount(t)
	account2 := createRandomAccount(t)
	fmt.Println(">>>> Initial Balances:", account1.Balance, account2.Balance)

	// Run it with goroutines
	n := 10 // Concurrent transactions
	amount := int64(10)

	errs := make(chan error)
	//results := make(chan TransferTxResult)

	for i := 0; i < n; i++ {
		fromAccountID := account1.ID
		toAccountID := account2.ID

		if i%2 == 1 {
			fromAccountID = account2.ID
			toAccountID = account1.ID
		}

		go func(i int, fromID, toID int64) {
			fmt.Printf("Starting deadlock test transaction %d: from %d to %d\n", i+1, fromID, toID)
			_, err := store.TransferTx(context.Background(), TransferTxParams{
				FromAccountID: fromID,
				ToAccountID:   toID,
				Amount:        amount,
			})
			if err != nil {
				fmt.Printf("Deadlock test transaction %d failed: %v\n", i+1, err)
			} else {
				fmt.Printf("Deadlock test transaction %d succeeded\n", i+1)
			}
			errs <- err
		}(i, fromAccountID, toAccountID)
	}

	// Check results
	for i := 0; i < n; i++ {
		err := <-errs
		require.NoError(t, err)
	}

	updatedAccount1, err := testQueries.GetAccount(context.Background(), account1.ID)
	require.NoError(t, err)

	updatedAccount2, err := testQueries.GetAccount(context.Background(), account2.ID)
	require.NoError(t, err)

	fmt.Println("<<<<< Final Balances:", updatedAccount1.Balance, updatedAccount2.Balance)
	require.Equal(t, account1.Balance, updatedAccount1.Balance)
	require.Equal(t, account2.Balance, updatedAccount2.Balance)
}
