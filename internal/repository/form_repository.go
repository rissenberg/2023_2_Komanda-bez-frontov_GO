package repository

import (
	"context"
	"fmt"
	"time"

	"go-form-hub/internal/database"
	"go-form-hub/internal/model"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type Form struct {
	Title     string    `db:"title"`
	ID        int64     `db:"id"`
	AuthorID  int64     `db:"author_id"`
	CreatedAt time.Time `db:"created_at"`
	Anonymous bool      `db:"anonymous"`
}

var (
	selectFields = []string{
		"f.id",
		"f.title",
		"f.created_at",
		"f.author_id",
		"f.anonymous",
		"u.id",
		"u.username",
		"u.first_name",
		"u.last_name",
		"u.email",
		"q.id",
		"q.title",
		"q.text",
		"q.type",
		"q.required",
		"a.id",
		"a.answer_text",
	}
)

var (
	selectFieldsFormInfo = []string{
		"f.id",
		"f.title",
		"f.created_at",
		"f.description",
		"f.anonymous",
		"u.id",
		"u.username",
		"u.first_name",
		"u.last_name",
		"u.email",
		"q.id",
		"q.title",
		"q.text",
		"q.type",
		"a.answer_text",
	}
	selectFieldsFormPassageInfo = []string{
		"fp.id",
		"ua.id",
		"ua.username",
		"ua.first_name",
		"ua.last_name",
		"ua.email",
		"q.id",
		"pa.answer_text",
	}
)

type formDatabaseRepository struct {
	db      database.ConnPool
	builder squirrel.StatementBuilderType
}

func NewFormDatabaseRepository(db database.ConnPool, builder squirrel.StatementBuilderType) FormRepository {
	return &formDatabaseRepository{
		db:      db,
		builder: builder,
	}
}

func (r *formDatabaseRepository) FindAll(ctx context.Context) (forms []*model.Form, err error) {
	query, _, err := r.builder.
		Select(selectFields...).
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.question as q ON q.form_id = f.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.answer as a ON a.question_id = q.id", r.db.GetSchema())).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to execute query: %e", err)
	}

	return r.fromRows(rows)
}

func (r *formDatabaseRepository) FormsSearch(ctx context.Context, title string, userID uint) (forms []*model.FormTitle, err error) {
	const limit = 5
	query := fmt.Sprintf(`select id, title, created_at
	FROM (select title, id, created_at, similarity(title, $1::text) as sim
	FROM %s.form
	WHERE author_id = $2::integer
	order by sim desc, created_at) as res
	LIMIT $3::integer`, r.db.GetSchema())

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_search failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, query, title, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_search failed to execute query: %e", err)
	}

	return r.searchTitleFromRows(rows)
}

// FormResults извлекает результаты формы по ID формы.
// Эта функция строит SQL-запрос для получения данных, связанных с формой,
// из базы данных, включая информацию о форме, вопросах, ответах и участниках.
// Результат представляет собой структурированный model.FormResult.
func (r *formDatabaseRepository) FormResults(ctx context.Context, id int64) (formResult *model.FormResult, err error) {
	formInfoQuery, formInfoArgs, err := r.builder.
		Select(selectFieldsFormInfo...).
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.question as q ON q.form_id = f.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.answer as a ON a.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"f.id": id}).
		ToSql()


	fmt.Println("SQL Query:", formInfoQuery)

	if err != nil {
		return nil, fmt.Errorf("form_repository form_results failed to build form info query: %e", err)
	}

	formPassageInfoQuery, formPassageInfoArgs, err := r.builder.
		Select(selectFieldsFormPassageInfo...).
		From(fmt.Sprintf("%s.form_passage as fp", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as ua ON fp.user_id = ua.id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.form_passage_answer as pa ON fp.id = pa.form_passage_id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.question as q ON pa.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"fp.form_id": id}).
		ToSql()

	fmt.Println("SQL Query:", formPassageInfoQuery)

	if err != nil {
		return nil, fmt.Errorf("form_repository form_passage_results failed to build form passage info query: %e", err)
	}

	formPassageCount, formPassageArgs, err := r.builder.
		Select("fp.form_id", "COUNT(DISTINCT fp.id) AS unique_response_count").
		From(fmt.Sprintf("%s.form_passage as fp", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as ua ON fp.user_id = ua.id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.form_passage_answer as pa ON fp.id = pa.form_passage_id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.question as q ON pa.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"fp.form_id": id}).
		GroupBy("fp.form_id").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("form_repository question_count failed to build form passage info query: %e", err)
	}

	questionPassageCount, questionPassageArgs, err := r.builder.
		Select("q.id", "COUNT(DISTINCT fp.id) AS unique_response_count").
		From(fmt.Sprintf("%s.form_passage as fp", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as ua ON fp.user_id = ua.id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.form_passage_answer as pa ON fp.id = pa.form_passage_id", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.question as q ON pa.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"fp.form_id": id}).
		GroupBy("q.id").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("form_repository question_count failed to build form passage info query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_results failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	countQuestion, err := tx.Query(ctx, questionPassageCount, questionPassageArgs...)
	if err != nil {
		return nil, fmt.Errorf("form_repository question_count failed to execute form info query: %e", err)
	}

	countQuestionResults, err := r.countQuestionFromRows(ctx, countQuestion)
	if err != nil {
		return nil, err
	}

	countForm, err := tx.Query(ctx, formPassageCount, formPassageArgs...)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_count failed to execute form info query: %e", err)
	}

	countFormResults, err := r.countFormFromRows(ctx, countForm)
	if err != nil {
		return nil, err
	}

	// Выполнение первого запроса
	rowsFormInfo, err := tx.Query(ctx, formInfoQuery, formInfoArgs...)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_results failed to execute form info query: %e", err)
	}

	// Обработка результатов первого запроса
	formResults, err := r.formResultsFromRows(ctx, rowsFormInfo)
	if err != nil {
		return nil, err
	}

	// Если результатов нет, возвращаем nil
	if len(formResults) == 0 {
		return nil, nil
	}

	// Выполнение второго запроса
	rowsFormPassageInfo, err := tx.Query(ctx, formPassageInfoQuery, formPassageInfoArgs...)
	if err != nil {
		return nil, fmt.Errorf("form_repository form_results failed to execute form passage info query: %e", err)
	}

	// Обработка результатов второго запроса
	formPassageResults, err := r.formPassageResultsFromRows(ctx, rowsFormPassageInfo)
	if err != nil {
		return nil, err
	}

	// Объединение результатов двух запросов
	for _, formPassageResult := range formPassageResults {
		formResult := formResults[0] // Поскольку у нас только один результат первого запроса, мы его берем
		for _, formCount := range countFormResults{
			if formCount.ID == formResult.ID {
				formResult.NumberOfPassagesForm = formCount.NumberOfPassagesForm
			}
		}
		// Поиск вопроса в результатах формы, к которому привязан результат прохождения
		for _, questionResult := range formResult.Questions {
			if questionResult.ID == formPassageResult.QuestionID {
				for _, questionCount := range countQuestionResults{
					if questionCount.ID == questionResult.ID {
						questionResult.NumberOfPassagesQuestion = questionCount.NumberOfPassagesQuestion
					}
				}
				answerExist := false
				for _, answerResult := range questionResult.Answers {
					if answerResult.Text == formPassageResult.AnswerText {
						answerResult.SelectedTimesAnswer++
						answerExist = true
						break
					}
				}
				if !answerExist {
					questionResult.Answers = append(questionResult.Answers, &model.AnswerResult{
						Text: formPassageResult.AnswerText,
						SelectedTimesAnswer: 1,
						// Добавьте другие поля ответа, если они есть
					})
				}
				
				//ToDO добавляем проверку на существование вопроса и тогда либо +1 либо далее(либо можно через типы)
				// Добавление ответа в соответствующий вопрос
				
			}
		}
		if !formResult.Anonymous {
			userExist := false
			for _, partisipantsResult := range formResult.Participants {
				if partisipantsResult.ID == formPassageResult.UserID {
					userExist = true
					break
				}
			}
			if !userExist {
				formResult.Participants = append(formResult.Participants, &model.UserGet{
					ID: formPassageResult.UserID,
					FirstName: formPassageResult.FirstName,
					Username: formPassageResult.Username,
					// Добавьте другие поля ответа, если они есть
				})
			}
		}
	}

	// Завершение транзакции
	return formResults[0], nil
}

// formResultsFromRows обрабатывает строки, полученные из результата запроса к базе данных,
// и создает список структурированных результатов формы.
// Она заполняет карты для организации вопросов и ответов по ID формы
// и вычисляет количество прохождений для каждой формы.
// Кроме того, она извлекает информацию о участниках для каждой формы,
// вызывая функцию getParticipantsForForm.
func (r *formDatabaseRepository) formResultsFromRows(ctx context.Context, rows pgx.Rows) ([]*model.FormResult, error) {
	defer func() {
		rows.Close()
	}()

	formResultMap := map[int64]*model.FormResult{}
	questionsByFormID := map[int64][]*model.QuestionResult{}
	answersByQuestionID := map[int64][]*model.AnswerResult{}

	for rows.Next() {
		info, err := r.formResultsFromRow(rows)
		if err != nil {
			return nil, err
		}

		if info.formResult == nil {
			continue
		}

		if _, ok := formResultMap[info.formResult.ID]; !ok {
			formResultMap[info.formResult.ID] = &model.FormResult{
				ID:                   info.formResult.ID,
				Title:                info.formResult.Title,
				Description:          info.formResult.Description,
				CreatedAt:            info.formResult.CreatedAt,
				Author:               info.formResult.Author,
				NumberOfPassagesForm: 0,
				Questions:            []*model.QuestionResult{},
				Anonymous:            info.formResult.Anonymous,
			}
		}

		var questionExists bool
		var existingQuestion *model.QuestionResult

		for _, existingQuestion = range formResultMap[info.formResult.ID].Questions {
			if existingQuestion.ID == info.questionResult.ID {
				questionExists = true
				break
			}
		}

		if questionExists {
			//existingQuestion.NumberOfPassagesQuestion++
			for _, existingAnswer := range existingQuestion.Answers {
				// if existingAnswer.Text == info.answerResult.Text {
				// 	existingAnswer.SelectedTimesAnswer++
				// 	break
				// } else {
					if existingAnswer == existingQuestion.Answers[len(existingQuestion.Answers)-1] {
						existingQuestion.Answers = append(existingQuestion.Answers, info.answerResult)
						if _, ok := answersByQuestionID[info.questionResult.ID]; !ok {
							answersByQuestionID[info.questionResult.ID] = make([]*model.AnswerResult, 0)
						}
						answersByQuestionID[info.questionResult.ID] = append(answersByQuestionID[info.questionResult.ID], info.answerResult)

				//	}
				}
			}
		} else {
			formResultMap[info.formResult.ID].Questions = append(formResultMap[info.formResult.ID].Questions, info.questionResult)
			info.questionResult.Answers = append(info.questionResult.Answers, info.answerResult)

			if _, ok := questionsByFormID[info.formResult.ID]; !ok {
				questionsByFormID[info.formResult.ID] = make([]*model.QuestionResult, 0)
			}

			questionsByFormID[info.formResult.ID] = append(questionsByFormID[info.formResult.ID], info.questionResult)

			if _, ok := answersByQuestionID[info.questionResult.ID]; !ok {
				answersByQuestionID[info.questionResult.ID] = make([]*model.AnswerResult, 0)
			}

			answersByQuestionID[info.questionResult.ID] = append(answersByQuestionID[info.questionResult.ID], info.answerResult)
		}
	}

	formResults := make([]*model.FormResult, 0, len(formResultMap))

	for _, formResult := range formResultMap {
		formResult.Questions = questionsByFormID[formResult.ID]
		for _, questionResult := range formResult.Questions {
			questionResult.Answers = answersByQuestionID[questionResult.ID]
		}

		// participants, err := r.getParticipantsForForm(ctx, formResult.ID)
		// if err != nil {
		// 	return nil, err
		// }
		// if !formResult.Anonymous {
		// 	formResult.Participants = participants
		// }

		formResults = append(formResults, formResult)
	}

	return formResults, nil
}

// getParticipantsForForm извлекает информацию о участниках (UserGet) для данной формы.
// Она строит SQL-запрос для выбора уникальных участников, которые ответили на вопросы в форме.
// Результат представляет собой слайс структурированных model.UserGet.
// func (r *formDatabaseRepository) getParticipantsForForm(ctx context.Context, formID int64) ([]*model.UserGet, error) {
// 	query, args, err := r.builder.
// 		Select("u.id", "MAX(u.username) as username", "MAX(u.first_name) as first_name", "MAX(u.last_name) as last_name", "MAX(u.email) as email").
// 		From(fmt.Sprintf("%s.passage_answer as pa", r.db.GetSchema())).
// 		Join(fmt.Sprintf("%s.user as u ON pa.user_id = u.id", r.db.GetSchema())).
// 		Join(fmt.Sprintf("%s.question as q ON pa.question_id = q.id", r.db.GetSchema())).
// 		Where(squirrel.Eq{"q.form_id": formID}).
// 		GroupBy("u.id").
// 		ToSql()

// 	if err != nil {
// 		return nil, fmt.Errorf("form_repository getParticipantsForForm failed to build query: %v", err)
// 	}

// 	tx, err := r.db.Begin(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("form_repository form_results failed to begin transaction: %e", err)
// 	}

// 	defer func() {
// 		switch err {
// 		case nil:
// 			err = tx.Commit(ctx)
// 		default:
// 			_ = tx.Rollback(ctx)
// 		}
// 	}()

// 	rows, err := tx.Query(ctx, query, args...)
// 	if err != nil {
// 		return nil, fmt.Errorf("form_repository form_results failed to execute query: %e", err)
// 	}

// 	defer rows.Close()

// 	participants := make([]*model.UserGet, 0)

// 	for rows.Next() {
// 		var participant model.UserGet
// 		if err := rows.Scan(&participant.ID, &participant.Username, &participant.FirstName, &participant.LastName, &participant.Email); err != nil {
// 			return nil, fmt.Errorf("form_repository getParticipantsForForm failed to scan row: %v", err)
// 		}
// 		participants = append(participants, &participant)
// 	}

// 	return participants, nil
// }

func (r *formDatabaseRepository) formPassageResultsFromRows(ctx context.Context, rows pgx.Rows) ([]*model.FormPassageResult, error) {
	defer func() {
		rows.Close()
	}()

	formPassageResults := make([]*model.FormPassageResult, 0)

	for rows.Next() {
		result := &model.FormPassageResult{}
		err := rows.Scan(
			&result.FormID,
			&result.UserID,
			&result.Username,
			&result.FirstName,
			&result.LastName,
			&result.Email,
			&result.QuestionID,
			&result.AnswerText,
		)
		if err != nil {
			return nil, fmt.Errorf("form_repository formPassageResultsFromRows failed to scan row: %v", err)
		}
		formPassageResults = append(formPassageResults, result)
	}

	return formPassageResults, nil
}

func (r *formDatabaseRepository) countQuestionFromRows(ctx context.Context, rows pgx.Rows) ([]*model.QuestionResult, error) {
	defer func() {
		rows.Close()
	}()

	countQuestionPassageResults := make([]*model.QuestionResult, 0)

	for rows.Next() {
		result := &model.QuestionResult{}
		err := rows.Scan(
			&result.ID,
			&result.NumberOfPassagesQuestion,
		)
		if err != nil {
			return nil, fmt.Errorf("form_repository countQuestionFromRows failed to scan row: %v", err)
		}
		countQuestionPassageResults = append(countQuestionPassageResults, result)
	}

	return countQuestionPassageResults, nil
}

func (r *formDatabaseRepository) countFormFromRows(ctx context.Context, rows pgx.Rows) ([]*model.FormResult, error) {
	defer func() {
		rows.Close()
	}()

	countFormPassageResults := make([]*model.FormResult, 0)

	for rows.Next() {
		result := &model.FormResult{}
		err := rows.Scan(
			&result.ID,
			&result.NumberOfPassagesForm,
		)
		if err != nil {
			return nil, fmt.Errorf("form_repository countFormFromRows failed to scan row: %v", err)
		}
		countFormPassageResults = append(countFormPassageResults, result)
	}

	return countFormPassageResults, nil
}

type formResultsFromRowReturn struct {
	formResult      *model.FormResult
	questionResult  *model.QuestionResult
	answerResult    *model.AnswerResult
}

// formResultsFromRowReturn представляет структурированные данные,
// возвращаемые при обработке одной строки в результате запроса к базе данных.
// Она включает информацию о форме, вопросе, ответе и участнике.
func (r *formDatabaseRepository) formResultsFromRow(row pgx.Row) (*formResultsFromRowReturn, error) {
	formResult := &model.FormResult{}
	questionResult := &model.QuestionResult{}
	answerResult := &model.AnswerResult{}
	formResult.Author = &model.UserGet{}

	err := row.Scan(
		&formResult.ID,
		&formResult.Title,
		&formResult.CreatedAt,
		&formResult.Description,
		&formResult.Anonymous,
		&formResult.Author.ID,
		&formResult.Author.Username,
		&formResult.Author.FirstName,
		&formResult.Author.LastName,
		&formResult.Author.Email,
		&questionResult.ID,
		&questionResult.Title,
		&questionResult.Description,
		&questionResult.Type,
		&answerResult.Text,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("form_repository failed to scan row: %e", err)
	}

	return &formResultsFromRowReturn{formResult, questionResult, answerResult}, nil
}

func (r *formDatabaseRepository) FindAllByUser(ctx context.Context, username string) (forms []*model.Form, err error) {
	query, args, err := r.builder.
		Select(selectFields...).
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.question as q ON q.form_id = f.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.answer as a ON a.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"u.username": username}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_all failed to execute query: %e", err)
	}

	return r.fromRows(rows)
}

func (r *formDatabaseRepository) FindByID(ctx context.Context, id int64) (form *model.Form, err error) {
	query, args, err := r.builder.
		Select(selectFields...).
		From(fmt.Sprintf("%s.form as f", r.db.GetSchema())).
		Join(fmt.Sprintf("%s.user as u ON f.author_id = u.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.question as q ON q.form_id = f.id", r.db.GetSchema())).
		LeftJoin(fmt.Sprintf("%s.answer as a ON a.question_id = q.id", r.db.GetSchema())).
		Where(squirrel.Eq{"f.id": id}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("form_repository find_by_title failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_by_title failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("form_repository find_by_title failed to execute query: %e", err)
	}

	forms, err := r.fromRows(rows)
	if len(forms) == 0 {
		return nil, nil
	}

	return forms[0], err
}

func (r *formDatabaseRepository) Insert(ctx context.Context, form *model.Form, tx pgx.Tx) (*model.Form, error) {
	var err error

	if tx == nil {
		tx, err = r.db.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("form_facade insert failed to begin transaction: %e", err)
		}
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	formQuery, args, err := r.builder.
		Insert(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Columns("title", "author_id", "created_at", "anonymous").
		Values(form.Title, form.Author.ID, form.CreatedAt, form.Anonymous).
		Suffix("RETURNING id").
		ToSql()
	err = tx.QueryRow(ctx, formQuery, args...).Scan(&form.ID)
	if err != nil {
		return nil, err
	}

	questionBatch := &pgx.Batch{}
	questionQuery := r.builder.
		Insert(fmt.Sprintf("%s.question", r.db.GetSchema())).
		Columns("title", "text", "type", "required", "form_id").
		Suffix("RETURNING id")

	for _, question := range form.Questions {
		q, args, err := questionQuery.Values(question.Title, question.Description, question.Type, question.Required, form.ID).ToSql()
		if err != nil {
			return nil, err
		}

		questionBatch.Queue(q, args...)
	}
	questionResults := tx.SendBatch(ctx, questionBatch)

	answerBatch := &pgx.Batch{}
	answerQuery := r.builder.
		Insert(fmt.Sprintf("%s.answer", r.db.GetSchema())).
		Columns("answer_text", "question_id").
		Suffix("RETURNING id")

	for _, question := range form.Questions {
		questionID := int64(0)
		err = questionResults.QueryRow().Scan(&questionID)
		if err != nil {
			return nil, err
		}

		question.ID = &questionID
		for _, answer := range question.Answers {
			q, args, err := answerQuery.Values(answer.Text, question.ID).ToSql()
			if err != nil {
				return nil, err
			}

			answerBatch.Queue(q, args...)
		}
	}
	questionResults.Close()

	answerResults := tx.SendBatch(ctx, answerBatch)
	for _, question := range form.Questions {
		for _, answer := range question.Answers {
			answerID := int64(0)
			err = answerResults.QueryRow().Scan(&answerID)
			if err != nil {
				return nil, err
			}

			answer.ID = &answerID
		}
	}
	answerResults.Close()

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return form, nil
}

func (r *formDatabaseRepository) FormPassageSave(ctx context.Context, formPassage *model.FormPassage, userID uint64) error {
	var err error

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("form_facade insert failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	formPassageQuery := fmt.Sprintf(`INSERT INTO %s.form_passage
	(user_id, form_id)
	VALUES($1::integer, $2::integer)
	RETURNING id`, r.db.GetSchema())

	var formPassageID int64
	err = tx.QueryRow(ctx, formPassageQuery, userID, formPassage.FormID).Scan(&formPassageID)
	if err != nil {
		return err
	}

	passageAnswerBatch := &pgx.Batch{}
	passageAnswerQuery := fmt.Sprintf(`INSERT INTO %s.form_passage_answer
	(answer_text, question_id, form_passage_id)
	VALUES($1::text, $2::integer, $3::integer)`, r.db.GetSchema())

	for _, passageAnswer := range formPassage.PassageAnswers {
		passageAnswerBatch.Queue(passageAnswerQuery, passageAnswer.Text,
			passageAnswer.QuestionID, formPassageID)
	}
	answerBatch := tx.SendBatch(ctx, passageAnswerBatch)
	answerBatch.Close()

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *formDatabaseRepository) FormPassageCount(ctx context.Context, formID int64) (int64, error) {
	var err error

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("form_facade insert failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	formPassageQuery := fmt.Sprintf(`select count(*)
	from %s.form_passage
	where form_id = $1`, r.db.GetSchema())

	var total int64
	err = tx.QueryRow(ctx, formPassageQuery, formID).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

func (r *formDatabaseRepository) Update(ctx context.Context, id int64, form *model.Form) (result *model.Form, err error) {
	query, args, err := r.builder.Update(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Set("title", form.Title).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING id, title, created_at").ToSql()
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("form_repository update failed to execute query: %e", err)
	}

	return form, nil
}

func (r *formDatabaseRepository) Delete(ctx context.Context, id int64) (err error) {
	query, args, err := r.builder.Delete(fmt.Sprintf("%s.form", r.db.GetSchema())).
		Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return fmt.Errorf("form_repository delete failed to build query: %e", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("form_repository delete failed to begin transaction: %e", err)
	}

	defer func() {
		switch err {
		case nil:
			err = tx.Commit(ctx)
		default:
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("form_repository delete failed to execute query: %e", err)
	}

	return nil
}

func (r *formDatabaseRepository) fromRows(rows pgx.Rows) ([]*model.Form, error) {
	defer func() {
		rows.Close()
	}()

	formMap := map[int64]*model.Form{}
	questionsByFormID := map[int64][]*model.Question{}
	answersByQuestionID := map[int64][]*model.Answer{}

	questionWasAppended := map[int64]bool{}

	for rows.Next() {
		info, err := r.fromRow(rows)
		if err != nil {
			return nil, err
		}

		if info.form == nil {
			continue
		}

		if _, ok := formMap[info.form.ID]; !ok {
			formMap[info.form.ID] = &model.Form{
				ID:        &info.form.ID,
				Title:     info.form.Title,
				CreatedAt: info.form.CreatedAt,
				Anonymous: info.form.Anonymous,
				Author: &model.UserGet{
					ID:        info.author.ID,
					Username:  info.author.Username,
					FirstName: info.author.FirstName,
					LastName:  info.author.LastName,
					Email:     info.author.Email,
					Avatar:    info.author.Avatar,
				},
			}
		}

		if _, ok := questionWasAppended[info.question.ID]; !ok {
			questionsByFormID[info.form.ID] = append(questionsByFormID[info.form.ID], &model.Question{
				ID:          &info.question.ID,
				Title:       info.question.Title,
				Description: info.question.Text,
				Type:        info.question.Type,
				Required:    info.question.Required,
			})
			questionWasAppended[info.question.ID] = true
		}

		if _, ok := answersByQuestionID[info.question.ID]; !ok {
			answersByQuestionID[info.question.ID] = make([]*model.Answer, 0, 1)
		}

		answersByQuestionID[info.question.ID] = append(answersByQuestionID[info.question.ID], &model.Answer{
			ID:   &info.answer.ID,
			Text: info.answer.AnswerText,
		})
	}

	forms := make([]*model.Form, 0, len(formMap))

	for _, form := range formMap {
		form.Questions = questionsByFormID[*form.ID]
		for _, question := range form.Questions {
			question.Answers = answersByQuestionID[*question.ID]
		}
		forms = append(forms, form)
	}

	return forms, nil
}

func (r *formDatabaseRepository) searchTitleFromRows(rows pgx.Rows) ([]*model.FormTitle, error) {
	defer func() {
		rows.Close()
	}()

	formTitleArray := make([]*model.FormTitle, 0)

	for rows.Next() {
		form, err := r.formTitleFromRow(rows)
		if err != nil {
			return nil, err
		}

		if form == nil {
			continue
		}

		formTitleArray = append(formTitleArray, &model.FormTitle{
			ID:        form.ID,
			Title:     form.Title,
			CreatedAt: form.CreatedAt,
		})
	}

	return formTitleArray, nil
}

type fromRowReturn struct {
	form     *Form
	author   *User
	question *Question
	answer   *Answer
}

func (r *formDatabaseRepository) formTitleFromRow(row pgx.Row) (*Form, error) {
	form := &Form{}

	err := row.Scan(
		&form.ID,
		&form.Title,
		&form.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("form_repository failed to scan row: %e", err)
	}

	return form, nil
}

func (r *formDatabaseRepository) fromRow(row pgx.Row) (*fromRowReturn, error) {
	form := &Form{}
	author := &User{}
	question := &Question{}
	answer := &Answer{}

	err := row.Scan(
		&form.ID,
		&form.Title,
		&form.CreatedAt,
		&form.AuthorID,
		&form.Anonymous,
		&author.ID,
		&author.Username,
		&author.FirstName,
		&author.LastName,
		&author.Email,
		&question.ID,
		&question.Title,
		&question.Text,
		&question.Type,
		&question.Required,
		&answer.ID,
		&answer.AnswerText,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("form_repository failed to scan row: %e", err)
	}

	return &fromRowReturn{form, author, question, answer}, nil
}
