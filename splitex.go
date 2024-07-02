package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type User struct {
	Id    int
	Name  string
	Email string
}

type Transaction struct {
	Id           int
	Amount       float64
	SplitBetween []*User
	CreatedBy    *User
	Timestamp    int64
}

type transactionGraphNode struct {
	FromPerson     *User
	RecieverPerson *User
	Amount         float64
}

type mainRouter struct {
	userList         *[]User
	countUser        *int
	transactionList  *[]Transaction
	countTransaction *int
	transactionLog   *[][]float64
	transactionGraph *[]transactionGraphNode
}

type transactionStruct struct {
	Error    string
	UserList []User
	UserId   int
}

type homeTransactionStruct struct {
	Id           int
	Amount       float64
	SplitBetween []*User
	CreatedBy    *User
	Timestamp    string
}

type userHomeStruct struct {
	UserObj      User
	Transaction  []transactionGraphNode
	AllTransactions []homeTransactionStruct
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("home.html")
	err := t.Execute(w, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func saveUser(w http.ResponseWriter, r *http.Request, userList *[]User, countUser *int) {
	fn := r.FormValue("fullname")
	email := r.FormValue("email")

	for _, each_user := range *userList {
		if each_user.Email == email {
			http.Redirect(w, r, "/user/"+strconv.Itoa(each_user.Id)+"/home", http.StatusSeeOther)
		}
	}

	*userList = append(*userList, User{
		Id:    (*countUser),
		Name:  fn,
		Email: email,
	})
	*countUser += 1

	http.Redirect(w, r, "/user/"+strconv.Itoa(*countUser-1)+"/home", http.StatusSeeOther)
}

func userHome(w http.ResponseWriter, r *http.Request, userObj User, transactionGraph *[]transactionGraphNode, allTrans *[]Transaction) {
	t, err := template.ParseFiles("user_home.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	userTransaction := []transactionGraphNode{}
	for _, each_node := range *transactionGraph {
		if userObj.Id == each_node.FromPerson.Id || userObj.Id == each_node.RecieverPerson.Id {
			userTransaction = append(userTransaction, each_node)
		}
	}

	userAllTrans := []homeTransactionStruct{}
	for _, each_t := range *allTrans {
		if userObj.Id == each_t.CreatedBy.Id {
			userAllTrans = append(userAllTrans, homeTransactionStruct{
				Id: each_t.Id,
				Amount: each_t.Amount,
				CreatedBy: each_t.CreatedBy,
				SplitBetween: each_t.SplitBetween,
				Timestamp: time.Unix(each_t.Timestamp, 0).UTC().Format(time.DateOnly),
			})

		}
	}

	userHomeObj := userHomeStruct{
		UserObj:     userObj,
		Transaction: userTransaction,
		AllTransactions: userAllTrans,
	}
	t.Execute(w, &userHomeObj)
}

func displayTransactionPage(w http.ResponseWriter, r *http.Request, userList []User) {
	t, err := template.ParseFiles("transaction.html")
	urlPath := r.URL.Path
	urlList := strings.Split(urlPath, "/")
	userId, _ := strconv.Atoi(urlList[2])
	print(err)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	transData := transactionStruct{
		Error:    "",
		UserList: userList,
		UserId:   userId,
	}
	if err != nil {
		transData.Error = err.Error()
	}
	t.Execute(w, &transData)
}

// populates transaction graph based on last transaction
func populateTransactionGraph(transactionGraph *[]transactionGraphNode, transaction [][]float64, userList *[]User) {
	*transactionGraph = (*transactionGraph)[:0]

	slices.SortFunc(transaction, func(a, b []float64) int {
		if a[0] > b[0] {
			return 1
		} else if a[0] < b[0] {
			return -1
		} else {
			return 0
		}
	})
	slices.Reverse(transaction)

	userMap := make(map[int]*User)
	for _, user_obj := range *userList {
		userMap[user_obj.Id] = &user_obj
	}
	fmt.Print(transaction)
	l := 0
	r := len(transaction) - 1
	for transaction[l][0] > 0 && transaction[r][0] < 0 {
		*transactionGraph = append(*transactionGraph, transactionGraphNode{
			FromPerson:     userMap[int(transaction[r][1])],
			RecieverPerson: userMap[int(transaction[l][1])],
			Amount:         min(transaction[l][0], -transaction[r][0]),
		})

		amtDeducted := min(transaction[l][0], -transaction[r][0])
		transaction[l][0] -= amtDeducted
		transaction[r][0] += amtDeducted

		slices.SortFunc(transaction, func(a, b []float64) int {
			if a[0] > b[0] {
				return 1
			} else if a[0] < b[0] {
				return -1
			} else {
				return 0
			}
		})
		slices.Reverse(transaction)
	}

	fmt.Print(*transactionGraph)
}

// create a new transaction
func createTransactionHandler(w http.ResponseWriter, r *http.Request, userObj User,
	transactionList *[]Transaction, countTransaction *int, transactionLog *[][]float64, userCount *int,
	transactionGraph *[]transactionGraphNode, allUsers *[]User) {

	Amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/addtransaction", http.StatusSeeOther)
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/addtransaction", http.StatusSeeOther)
		return
	}

	SplitBetweenStr := r.Form["split_between"]
	SplitBetween := []*User{}
	if len(SplitBetweenStr) < 1 {
		err := errors.New("please select atleast one person to split this transaction with")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/addtransaction", http.StatusSeeOther)
		return
	}

	mapUsers := make(map[int]*User)
	for _, user := range *allUsers {
		mapUsers[user.Id] = &user
	}

	for _, each_user := range SplitBetweenStr {
		temp, err := strconv.Atoi(each_user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/addtransaction", http.StatusSeeOther)
			return
		}
		if mapUsers[temp] != nil {
			SplitBetween = append(SplitBetween, mapUsers[temp])
		}
	}

	if Amount <= 0 {
		err := errors.New("amount can't be negative")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/addtransaction", http.StatusSeeOther)
		return
	}

	*transactionList = append(*transactionList,
		Transaction{
			Id:           *countTransaction,
			Amount:       Amount,
			SplitBetween: SplitBetween,
			CreatedBy:    &userObj,
			Timestamp:    time.Now().Unix(),
		})

	numofMembers := len(SplitBetween)
	newLog := []float64{}
	if len(*transactionLog) > 0 {
		newLog = (*transactionLog)[len(*transactionLog)-1]

	} else {
		for i := 0; i < *userCount; i++ {
			newLog = append(newLog, 0)
		}
	}

	*countTransaction += 1
	newLog[userObj.Id] += Amount
	for _, each_member := range SplitBetween {
		newLog[each_member.Id] -= (Amount / float64(numofMembers))
	}

	*transactionLog = append(*transactionLog, newLog)
	fmt.Print(*transactionLog)
	modTransactionList := [][]float64{}
	for i, amt := range newLog {
		modTransactionList = append(modTransactionList, []float64{
			amt, float64(i),
		})
	}

	populateTransactionGraph(transactionGraph, modTransactionList, allUsers)
	http.Redirect(w, r, "/user/"+strconv.Itoa(userObj.Id)+"/home", http.StatusSeeOther)
}

func (mr *mainRouter) mainRouter(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	pathExp := regexp.MustCompile(`/user/\d/home`)
	transExp := regexp.MustCompile(`/user/\d/addtransaction`)
	transcrExp := regexp.MustCompile(`/user/\d/createtransaction`)

	if urlPath == "/register" || urlPath == "/" {
		indexHandler(w, r)
	} else if urlPath == "/user/save" {
		saveUser(w, r, mr.userList, mr.countUser)
	} else if pathExp.MatchString(urlPath) {
		urlList := strings.Split(urlPath, "/")
		userId, _ := strconv.Atoi(urlList[2])
		userObj := User{}
		for _, each_user := range *mr.userList {
			if each_user.Id == userId {
				userObj = each_user
			}
		}
		userHome(w, r, userObj, mr.transactionGraph, mr.transactionList)
	} else if transExp.MatchString(urlPath) {
		displayTransactionPage(w, r, *mr.userList)
	} else if transcrExp.MatchString(urlPath) {
		urlList := strings.Split(urlPath, "/")
		userId, _ := strconv.Atoi(urlList[2])
		userObj := User{}
		for _, each_user := range *mr.userList {
			if each_user.Id == userId {
				userObj = each_user
			}
		}
		createTransactionHandler(w, r, userObj, mr.transactionList,
			mr.countTransaction, mr.transactionLog, mr.countUser, mr.transactionGraph,
			mr.userList)
	}
}

// main function
func main() {
	allUsers := []User{}
	transactionList := []Transaction{}
	countUsers := 0
	countTransaction := 0
	transactionLog := [][]float64{}
	transactionGraph := []transactionGraphNode{}

	mainHandler := &mainRouter{userList: &allUsers, countUser: &countUsers,
		transactionList: &transactionList, countTransaction: &countTransaction,
		transactionLog: &transactionLog, transactionGraph: &transactionGraph}

	http.HandleFunc("/", mainHandler.mainRouter)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
